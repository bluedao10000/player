import StoreKit
import StoreKitTest
import XCTest

@testable import AnimeApp

final class MembershipTests: XCTestCase {
    @MainActor
    func testBothProductsCarryReaderIdentityThroughPurchase() async throws {
        let bundle = Bundle(for: Self.self)
        let file = try XCTUnwrap(bundle.url(forResource: "Folio", withExtension: "storekit"))
        let session = try SKTestSession(contentsOf: file)
        session.disableDialogs = true
        session.clearTransactions()
        defer { session.clearTransactions() }

        let membership = StoreMembership()
        await membership.load()
        XCTAssertEqual(membership.products.map(\.id), StoreMembership.identifiers)
        let owner = UUID()
        for product in membership.products {
            let purchase = try await product.purchase(options: [.appAccountToken(owner)])
            guard case .success(.verified(let transaction)) = purchase else {
                return XCTFail("Expected a verified local transaction for " + product.id)
            }
            XCTAssertEqual(transaction.appAccountToken, owner)
            XCTAssertEqual(transaction.productType, .nonRenewable)
            await transaction.finish()
        }
        let products = try await Product.products(for: [CommissionPurchase.productID])
        let topup = try XCTUnwrap(products.first)
        XCTAssertEqual(topup.type, .consumable)
        let wallet = UUID()
        guard case .success(.verified(let payment)) = try await topup.purchase(options: [.appAccountToken(wallet)]) else {
            return XCTFail("Expected a verified commission top-up")
        }
        XCTAssertEqual(payment.appAccountToken, wallet)
        await payment.finish()
        XCTAssertEqual(session.allTransactions().count, 3)
    }
}
