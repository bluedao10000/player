import AuthenticationServices
import CryptoKit
import SwiftUI
@preconcurrency import WechatOpenSDK

@MainActor final class WechatLogin: NSObject, @preconcurrency WXApiDelegate {
  static let shared = WechatLogin()
  static var configured: Bool {
    let id = Bundle.main.object(forInfoDictionaryKey: "FolioWechatAppID") as? String
    let link = Bundle.main.object(forInfoDictionaryKey: "FolioWechatLink") as? String
    return id?.hasPrefix("wx") == true && link?.hasPrefix("https://") == true
  }
  private var response: CheckedContinuation<String, Error>?
  private var state: String?
  private var timeout: Task<Void, Never>?
  private func finish(_ result: Result<String, Error>) {
    timeout?.cancel()
    let reply = response
    response = nil
    state = nil
    reply?.resume(with: result)
  }
  func authorize() async throws -> String {
    guard response == nil else { throw AccountError.message("微信授权正在进行中。") }
    guard let app = Bundle.main.object(forInfoDictionaryKey: "FolioWechatAppID") as? String,
      app.hasPrefix("wx"),
      let link = Bundle.main.object(forInfoDictionaryKey: "FolioWechatLink") as? String,
      link.hasPrefix("https://"),
      WXApi.registerApp(app, universalLink: link)
    else { throw AccountError.message("微信登录暂不可用。") }
    guard WXApi.isWXAppInstalled() else { throw AccountError.message("请在这台 iPhone 上安装微信后重试。") }
    let challenge = UUID().uuidString
    let req = SendAuthReq()
    req.scope = "snsapi_userinfo"
    req.state = challenge
    return try await withCheckedThrowingContinuation { continuation in
      response = continuation
      state = challenge
      timeout = Task {
        try? await Task.sleep(for: .seconds(180))
        if !Task.isCancelled && state == challenge {
          finish(.failure(AccountError.message("微信授权已过期。")))
        }
      }
      WXApi.send(req) { ok in
        Task { @MainActor in
          if !ok && self.state == challenge {
            self.finish(.failure(AccountError.message("无法打开微信。")))
          }
        }
      }
    }
  }
  func onResp(_ resp: BaseResp) {
    guard let auth = resp as? SendAuthResp, response != nil else { return }
    guard auth.errCode == 0, auth.state == state, let code = auth.code, !code.isEmpty else {
      finish(.failure(AccountError.message("微信授权已取消或无效。")))
      return
    }
    finish(.success(code))
  }
  func handle(_ url: URL) { _ = WXApi.handleOpen(url, delegate: self) }
  func handle(_ activity: NSUserActivity) {
    _ = WXApi.handleOpenUniversalLink(activity, delegate: self)
  }
}

struct ReaderAccountPanel: View {
  @Environment(Shelf.self) private var shelf
  @State private var nonce: String?
  @State private var email = ""
  @State private var code = ""
  @State private var sent = false
  @State private var busy = false
  private func action(_ job: @escaping @MainActor () async throws -> Void) {
    guard !busy else { return }
    busy = true
    Task {
      defer { busy = false }
      do { try await job() } catch {
        if !(error is CancellationError) { shelf.issue = error.localizedDescription }
      }
    }
  }
  private func challenge() async {
    struct Challenge: Decodable { let nonce: String }
    do {
      nonce = try JSONDecoder().decode(
        Challenge.self, from: try await shelf.call("native/challenge")
      ).nonce
    } catch { nonce = nil }
  }
  var body: some View {
    VStack(alignment: .leading, spacing: 14) {
      if let user = shelf.user {
        Text(user.email ?? "读者账户").font(.headline)
        if user.pending {
          Text("首次登录，要带上这台设备的书架和阅读进度吗？")
          Button("合并到我的账户") { action { try await shelf.decide(true) } }.buttonStyle(
            .borderedProminent)
          Button("跳过，仅保留在本机") { action { try await shelf.decide(false) } }
        } else {
          MembershipPanel()
          DisclosureGroup("账户与绑定") { signInControls }
          Button("退出登录") { action { try await shelf.logout() } }
        }
      } else if shelf.token != nil {
        Button("重新连接读者账户") { action { try await shelf.refresh() } }
      } else {
        Text("登录以同步书架和阅读进度").font(.headline)
        signInControls
      }
    }.disabled(busy)
      .task(id: shelf.user?.id) { await challenge() }
  }

  @ViewBuilder private var signInControls: some View {
    if shelf.user?.appleLinked != true {
      SignInWithAppleButton(shelf.user == nil ? .signIn : .continue) { req in
        if let nonce {
          req.nonce = SHA256.hash(data: Data(nonce.utf8)).map { String(format: "%02x", $0) }
            .joined()
        }
      } onCompletion: { result in
        guard let nonce else { return }
        action {
          defer { Task { await challenge() } }
          let credential = try result.get().credential as? ASAuthorizationAppleIDCredential
          guard let bytes = credential?.identityToken,
            let token = String(data: bytes, encoding: .utf8)
          else { throw AccountError.message("未收到 Apple 授权。") }
          try await shelf.login(
            shelf.user == nil ? "native/apple" : "native/link/apple",
            body: ["token": token, "nonce": nonce, "bundle": Bundle.main.bundleIdentifier ?? ""])
        }
      }.signInWithAppleButtonStyle(.white).frame(height: 46).disabled(nonce == nil)
      if nonce == nil { Button("重试 Apple 登录") { Task { await challenge() } } }
    }
    if WechatLogin.configured && shelf.user?.mobileWechatLinked != true {
      Button(shelf.user == nil ? "微信登录" : "绑定微信") {
        action {
          let code = try await WechatLogin.shared.authorize()
          try await shelf.login(
            shelf.user == nil ? "native/wechat" : "native/link/wechat", body: ["code": code])
        }
      }.buttonStyle(.bordered)
    }
    if shelf.user?.email == nil {
      DisclosureGroup(shelf.user == nil ? "邮箱验证码登录" : "绑定邮箱") {
        TextField("邮箱", text: $email).keyboardType(.emailAddress).textContentType(.emailAddress)
          .textInputAutocapitalization(.never).disabled(sent)
        if sent {
          TextField("6 位验证码", text: $code).keyboardType(.numberPad).textContentType(.oneTimeCode)
          Button("确认") {
            action {
              try await shelf.login(
                shelf.user == nil ? "verify" : "email/link", body: ["email": email, "code": code])
              sent = false
              code = ""
            }
          }
          Button("更换邮箱或重新发送") {
            sent = false
            code = ""
          }
        } else {
          Button("发送验证码") {
            action {
              _ = try await shelf.call("code", ["email": email])
              sent = true
            }
          }
        }
      }
    }
  }
}
