import StoreKit
import SwiftUI

@MainActor @Observable final class StoreMembership {
  static let plus = "com.kmszz.starWiki.plus.30days"
  static let pro = "com.kmszz.starWiki.pro.30days"
  static let identifiers = [plus, pro]
  var products = [Product]()
  var busy = false
  var message: String?
  func load() async {
    do {
      products = try await Product.products(for: Self.identifiers).filter {
        $0.type == .nonRenewable
      }.sorted { $0.price < $1.price }
      message = products.count == 2 ? nil : "部分会员套餐暂时无法购买。"
    } catch { message = error.localizedDescription }
  }
  func deliver(_ result: VerificationResult<StoreKit.Transaction>, shelf: Shelf) async throws {
    guard case .verified(let tx) = result else { throw AccountError.message("购买未能完成验证，请稍后在「恢复购买」中重试。") }
    guard Self.identifiers.contains(tx.productID), let owner = shelf.user?.id,
      let id = UUID(uuidString: owner), tx.appAccountToken == id
    else { return }
    _ = try await shelf.call(
      "native/store/receipt", ["signedTransaction": result.jwsRepresentation])
    await tx.finish()
    try await shelf.refresh()
  }
  func buy(_ p: Product, shelf: Shelf) async {
    guard !busy, let owner = shelf.user?.id, let id = UUID(uuidString: owner) else { return }
    busy = true
    defer { busy = false }
    do {
      try await shelf.refresh()
      guard shelf.user?.id == owner, shelf.member?.plan == "free" else {
        throw AccountError.message("当前会员已生效。")
      }
      for await result in StoreKit.Transaction.unfinished {
        if case .verified(let tx) = result, tx.appAccountToken == id,
          Self.identifiers.contains(tx.productID)
        {
          try await deliver(result, shelf: shelf)
          message = "已同步之前的购买，请核对会员权益。"
          return
        }
      }
      switch try await p.purchase(options: [.appAccountToken(id)]) {
      case .success(let result):
        guard shelf.user?.id == owner else { throw AccountError.message("请登录发起购买的读者账户以领取会员。") }
        try await deliver(result, shelf: shelf)
        message = "购买已同步。"
      case .pending: message = "付款待确认，确认后会自动入账。"
      case .userCancelled: message = nil
      @unknown default: message = "请稍后查看购买记录。"
      }
    } catch { message = error.localizedDescription }
  }
  func restore(_ shelf: Shelf) async {
    guard !busy else { return }
    busy = true
    defer { busy = false }
    do {
      try await AppStore.sync()
      for await t in StoreKit.Transaction.all {
        if case .verified(let tx) = t, Self.identifiers.contains(tx.productID) {
          try await deliver(t, shelf: shelf)
        }
      }
      try await shelf.refresh()
      message = "已核对当前账户的购买记录。"
    } catch { message = error.localizedDescription }
  }
  func observe(_ shelf: Shelf) async {
    for await r in StoreKit.Transaction.unfinished {
      do { try await deliver(r, shelf: shelf) } catch { message = error.localizedDescription }
    }
    for await r in StoreKit.Transaction.updates {
      do { try await deliver(r, shelf: shelf) } catch { message = error.localizedDescription }
    }
  }
}
struct MembershipPanel: View {
  @Environment(Shelf.self) private var shelf
  @Environment(StoreMembership.self) private var store
  // 付费档写「Plus 会员」「Pro 会员」，免费档只写「Free」。
  private var planTitle: String {
    let plan = shelf.member?.plan ?? shelf.user?.plan ?? "free"
    return plan == "free" ? "Free" : plan.capitalized + " 会员"
  }
  // 档位积分以服务端会员接口为准。
  private var planNote: String {
    let paid = (shelf.member?.plans ?? []).filter { $0.id != "free" }
      .map { "\($0.id.capitalized) 每期 \($0.credits) 积分" }
    return (paid.isEmpty ? "" : paid.joined(separator: "，") + "。") + "单次购买 30 天，到期不自动续费。"
  }
  var body: some View {
    ReaderSection(title: planTitle) {
      if let m = shelf.member {
        Text("本期可用 \(max(0, Int((Double(m.credits) - Double(m.usedMicro) / 1e6).rounded(.down)))) / \(m.credits) 积分")
        if let end = try? Date(m.periodEnd, strategy: .iso8601) {
          Text("有效期至 " + end.formatted(date: .abbreviated, time: .omitted)).foregroundStyle(
            .secondary)
        }
      }
      Text(planNote).font(.footnote)
        .foregroundStyle(.secondary)
      ForEach(store.products, id: \.id) { p in
        Button("\(p.id==StoreMembership.plus ? "Plus" : "Pro") · \(p.displayPrice) / 30 天") {
          Task { await store.buy(p, shelf: shelf) }
        }.buttonStyle(.bordered).disabled(store.busy || shelf.member?.plan != "free")
      }
      if let orders = shelf.member?.storeOrders, !orders.isEmpty {
        DisclosureGroup("App Store 购买记录") {
          ForEach(orders) { order in
            VStack(alignment: .leading) {
              Text(order.plan.capitalized + (order.refunded ? " · 已退款" : " · 已付款"))
              if let price = order.price_milli, let currency = order.currency, !currency.isEmpty {
                Text((Decimal(price) / 1000).formatted(.currency(code: currency)))
              }
              Text(String(order.purchased_at.prefix(10))).foregroundStyle(.secondary)
            }.frame(maxWidth: .infinity, alignment: .leading)
          }
        }
      }
      if let msg = store.message { Text(msg).font(.footnote) }
      if store.products.count < 2 { Button("重试") { Task { await store.load() } } }
      Button("恢复购买") { Task { await store.restore(shelf) } }.disabled(store.busy)
    }.task { await store.load() }
  }
}
