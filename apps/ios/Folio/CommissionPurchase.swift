import Foundation
import StoreKit

@MainActor
enum CommissionPurchase {
  static let productID = "com.kmszz.starWiki.commission.20"

  static func deliver(_ receipt: VerificationResult<Transaction>) async throws {
    guard case .verified(let transaction) = receipt, transaction.productID == productID,
      transaction.appAccountToken != nil, transaction.productType == .consumable
    else { throw AccountError.message("约稿充值交易无效。") }

    let url = LibraryAPI.accounts.appending(path: "ios-account/native/store/wallet")
    var request = URLRequest(url: url)
    request.httpMethod = "POST"
    request.timeoutInterval = 20
    request.httpShouldHandleCookies = false
    request.setValue("application/json", forHTTPHeaderField: "Content-Type")
    request.httpBody = try JSONEncoder().encode(["signedTransaction": receipt.jwsRepresentation])
    let (_, response) = try await URLSession.shared.data(for: request)
    guard (response as? HTTPURLResponse)?.statusCode == 204 else {
      throw AccountError.message("充值尚未入账，返回约稿页重试即可核对原交易。")
    }
    await transaction.finish()
  }

  static func buy(wallet: UUID) async throws -> String? {
    for await receipt in Transaction.unfinished {
      if case .verified(let transaction) = receipt,
        transaction.productID == productID, transaction.appAccountToken == wallet {
        try await deliver(receipt)
        return "之前的充值已入账。"
      }
    }
    guard let product = try await Product.products(for: [productID]).first,
      product.type == .consumable else { throw AccountError.message("当前商店暂未提供约稿充值。") }
    switch try await product.purchase(options: [.appAccountToken(wallet)]) {
    case .success(let receipt):
      try await deliver(receipt)
      return "¥20 已计入约稿余额。"
    case .pending: return "付款待确认，确认后会自动入账。"
    case .userCancelled: return nil
    @unknown default: return "请稍后查看约稿流水。"
    }
  }
}
