import Foundation
import Security
import SwiftUI

enum AccountError: LocalizedError {
  case message(String)
  var errorDescription: String? {
    switch self {
    case .message(let value): value
    }
  }
}

private enum SessionVault {
  private static var vaultSuffix: String {
    #if FOLIO_SANDBOX
    return ".sandbox"
    #else
    return ""
    #endif
  }
  static var query: [String: Any] {
    [
      kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: (Bundle.main.bundleIdentifier ?? "Folio") + ".reader" + vaultSuffix,
      kSecAttrAccount as String: "session",
    ]
  }
  static func read() -> String? {
    var q = query
    q[kSecReturnData as String] = true
    var item: CFTypeRef?
    guard SecItemCopyMatching(q as CFDictionary, &item) == errSecSuccess, let data = item as? Data
    else { return nil }
    return String(data: data, encoding: .utf8)
  }
  static func write(_ value: String?) throws {
    guard let value else {
      let status = SecItemDelete(query as CFDictionary)
      guard status == errSecSuccess || status == errSecItemNotFound else {
        throw AccountError.message("无法清除登录凭据。")
      }
      return
    }
    let bytes = Data(value.utf8)
    let status = SecItemUpdate(
      query as CFDictionary, [kSecValueData as String: bytes] as CFDictionary)
    if status == errSecItemNotFound {
      var q = query
      q[kSecValueData as String] = bytes
      q[kSecAttrAccessible as String] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
      if SecItemAdd(q as CFDictionary, nil) == errSecSuccess { return }
    } else if status == errSecSuccess {
      return
    }
    throw AccountError.message("无法安全保存登录凭据。")
  }
}

struct RemoteRecord: Codable {
  let href, title, work: String
  let anchor, excerpt: String?
  let percent: Int?
  let recordedAt: Int64
  var chapter: SavedChapter? {
    let parts = href.split(separator: "/").map(String.init)
    let world: String
    let slug: String
    if parts.count == 2, parts[0] == "story" {
      world = "tidepost"
      slug = parts[1]
    } else if parts.count == 4, parts[0] == "work", parts[2] == "chapter" {
      world = parts[1]
      slug = parts[3]
    } else {
      return nil
    }
    return SavedChapter(
      work: world, slug: slug, title: title, worldTitle: work, anchor: anchor, href: href,
      percent: percent, excerpt: excerpt, recordedAt: recordedAt)
  }
}
struct ReaderState: Codable {
  let id: String
  let email: String?
  let plan: String
  let pending, wechatLinked: Bool
  let appleLinked, mobileWechatLinked: Bool?
  let bookmarks, positions: [RemoteRecord]
}
struct MemberInfo: Decodable {
  let plan, periodEnd: String
  let credits: Int
  struct StoreOrder: Decodable, Identifiable {
    let id, plan, purchased_at: String
    let currency: String?
    let price_milli: Int64?
    let refunded: Bool
  }
  let storeOrders: [StoreOrder]?
  let usedMicro: Int64
}
@MainActor @Observable final class Shelf {
  struct Records: Codable {
    var saved: [SavedChapter] = []
    var recent: [SavedChapter] = []
  }
  private struct Write: Codable, Identifiable {
    var id = UUID()
    let item: SavedChapter
    let saved: Bool?
  }
  private struct Cache: Codable {
    var records = Records()
    var writes: [Write] = []
  }
  private(set) var records: Records
  private(set) var user: ReaderState?
  private(set) var member: MemberInfo?
  private(set) var token: String?
  var issue: String?
  private let defaults = UserDefaults.standard
  private let guestKey = "folio.native.shelf.v1"
  private var cache = Cache()
  private var syncing = false
  private var scheduled: Task<Void, Never>?
  init() {
    records = Self.restore(Records.self, key: "folio.native.shelf.v1") ?? Records()
    token = SessionVault.read()
  }
  private static func restore<T: Decodable>(_ type: T.Type, key: String) -> T? {
    UserDefaults.standard.data(forKey: key).flatMap { try? JSONDecoder().decode(type, from: $0) }
  }
  func call(_ path: String, _ body: [String: Any]? = nil) async throws -> Data {
    var req = URLRequest(url: LibraryAPI.accounts.appending(path: "ios-account/" + path))
    req.timeoutInterval = 20
    req.httpShouldHandleCookies = false
    let usedToken = token
    if let usedToken { req.setValue("Bearer " + usedToken, forHTTPHeaderField: "Authorization") }
    if let user { req.setValue(user.id, forHTTPHeaderField: "X-Reader-ID") }
    if let body {
      req.httpMethod = "POST"
      req.httpBody = try JSONSerialization.data(withJSONObject: body)
      req.setValue("application/json", forHTTPHeaderField: "Content-Type")
    }
    let (data, res) = try await URLSession.shared.data(for: req)
    guard let http = res as? HTTPURLResponse else { throw URLError(.badServerResponse) }
    guard token == usedToken else { throw CancellationError() }
    if http.statusCode == 401 && usedToken != nil && !path.hasPrefix("native/link/") { try leave() }
    guard (200..<300).contains(http.statusCode) else {
      let error = (try? JSONSerialization.jsonObject(with: data)) as? [String: String]
      throw AccountError.message(error?["error"] ?? "请求暂时无法完成（\(http.statusCode)）。")
    }
    return data
  }
  func login(_ path: String, body: [String: Any]) async throws {
    let data = try await call(path, body)
    if token == nil {
      struct Grant: Decodable { let token: String }
      let value = try JSONDecoder().decode(Grant.self, from: data).token
      guard value.count == 64 else { throw URLError(.cannotParseResponse) }
      try SessionVault.write(value)
      token = value
    }
    try await refresh()
  }
  func refresh() async throws {
    guard token != nil else { return }
    let updated = try JSONDecoder().decode(ReaderState.self, from: try await call("state"))
    if user?.id != updated.id {
      cache = Self.restore(Cache.self, key: cacheKey(updated.id)) ?? Cache()
    }
    user = updated
    if !updated.pending {
      records = Records(
        saved: updated.bookmarks.compactMap(\.chapter),
        recent: updated.positions.compactMap(\.chapter))
      for w in cache.writes { apply(w) }
      persist()
      schedule()
    }
    member = try JSONDecoder().decode(MemberInfo.self, from: try await call("membership"))
  }
  func decide(_ merge: Bool) async throws {
    let guest = Self.restore(Records.self, key: guestKey) ?? Records()
    _ = try await call(
      "merge",
      [
        "choice": merge ? "merge" : "skip", "bookmarks": merge ? guest.saved.map(payload) : [],
        "positions": merge ? guest.recent.map(payload) : [],
      ])
    try await refresh()
  }
  func logout() async throws {
    if !cache.writes.isEmpty { try await flush() }
    if token != nil { _ = try await call("logout", [:]) }
    try leave()
  }
  private func leave() throws {
    try SessionVault.write(nil)
    scheduled?.cancel()
    token = nil
    user = nil
    member = nil
    cache = Cache()
    records = Self.restore(Records.self, key: guestKey) ?? Records()
  }
  private func cacheKey(_ id: String) -> String { "folio.account." + id }
  private func payload(_ item: SavedChapter) -> [String: Any] {
    [
      "href": item.canonical, "title": item.title, "work": item.worldTitle,
      "anchor": item.anchor ?? "",
      "percent": item.percent ?? 0, "excerpt": item.excerpt ?? "",
      "recordedAt": item.recordedAt ?? 1,
    ]
  }
  private func apply(_ w: Write) {
    if let saved = w.saved {
      records.saved.removeAll { $0.id == w.item.id }
      if saved { records.saved.insert(w.item, at: 0) }
    } else {
      records.recent.removeAll { $0.id == w.item.id }
      records.recent.insert(w.item, at: 0)
      records.recent = Array(records.recent.prefix(100))
    }
  }
  func toggle(_ item: SavedChapter) {
    update(item, saved: !records.saved.contains { $0.id == item.id })
  }
  func remember(_ item: SavedChapter) {
    if records.recent.first?.id == item.id && records.recent.first?.anchor == item.anchor { return }
    update(item, saved: nil)
  }
  private func update(_ original: SavedChapter, saved: Bool?) {
    if token != nil && user == nil { return }
    var item = original
    item.recordedAt = Int64(Date().timeIntervalSince1970 * 1000)
    let w = Write(item: item, saved: saved)
    apply(w)
    if let user, !user.pending {
      cache.writes.removeAll { $0.item.id == item.id && ($0.saved == nil) == (saved == nil) }
      cache.writes.append(w)
    }
    persist()
    schedule()
  }
  private func persist() {
    if let user, !user.pending {
      cache.records = records
      if let d = try? JSONEncoder().encode(cache) { defaults.set(d, forKey: cacheKey(user.id)) }
    } else if let d = try? JSONEncoder().encode(records) {
      defaults.set(d, forKey: guestKey)
    }
  }
  private func schedule() {
    guard !cache.writes.isEmpty, !syncing else { return }
    scheduled?.cancel()
    scheduled = Task {
      do {
        try await Task.sleep(for: .milliseconds(300))
        try await flush()
      } catch is CancellationError {} catch { issue = error.localizedDescription }
    }
  }
  func flush() async throws {
    guard !syncing, let owner = user?.id else { return }
    syncing = true
    defer { syncing = false }
    while let w = cache.writes.first, user?.id == owner {
      var body = payload(w.item)
      if let saved = w.saved { body["saved"] = saved }
      _ = try await call(w.saved == nil ? "progress" : "bookmark", body)
      guard user?.id == owner else { return }
      cache.writes.removeAll { $0.id == w.id }
      persist()
    }
  }
}
