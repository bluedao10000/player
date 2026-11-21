import XCTest

final class ReadingTests: XCTestCase {
    @MainActor
    func testPublishedReadingJourney() throws {
        let app = XCUIApplication()
        if let site = ProcessInfo.processInfo.environment["FOLIO_SITE_URL"] {
            app.launchEnvironment["FOLIO_SITE_URL"] = site
        }
        app.launch()
        let world = app.buttons["world-tidepost"]
        XCTAssertTrue(world.waitForExistence(timeout: 20), "需要可访问的已发布作品接口")
        capture("发现", app)
        world.tap()
        let mio = app.buttons["entry-mio"]
        reveal(mio, app)
        mio.tap()
        let chapter = app.buttons["entry-last-ferry"]
        reveal(chapter, app)
        chapter.tap()
        let save = app.buttons["收藏章节"]
        let remove = app.buttons["移出收藏"]
        XCTAssertTrue(save.waitForExistence(timeout: 5) || remove.exists)
        if save.exists { save.tap() }
        let scene = app.buttons["entry-ferry-register"]
        reveal(scene, app)
        scene.tap()
        let back = app.buttons["回到原文"]
        XCTAssertTrue(back.waitForExistence(timeout: 8))
        back.tap()
        let paragraph = app.staticTexts["passage-missing-ticket"]
        XCTAssertTrue(paragraph.waitForExistence(timeout: 8))
        XCTAssertTrue(paragraph.isHittable)
        XCTAssertLessThan(paragraph.frame.minY, app.frame.height / 3)
        capture("回到原文", app)
        app.terminate()
        app.launch()
        app.tabBars.buttons["我的"].tap()
        let resume = app.buttons["continue-reading"]
        XCTAssertTrue(resume.waitForExistence(timeout: 8))
        XCTAssertTrue(resume.label.contains("末班渡船"))
        resume.tap()
        XCTAssertTrue(remove.waitForExistence(timeout: 8))
        XCTAssertTrue(paragraph.isHittable)
        XCTAssertLessThan(paragraph.frame.minY, app.frame.height / 3)
    }

    @MainActor private func reveal(_ element: XCUIElement, _ app: XCUIApplication) {
        XCTAssertTrue(element.waitForExistence(timeout: 8))
        for _ in 0..<10 {
            if element.isHittable && app.frame.insetBy(dx: 0, dy: 95).contains(element.frame.center) { return }
            app.swipeUp()
        }
        XCTAssertTrue(element.isHittable)
    }
    @MainActor private func capture(_ name: String, _ app: XCUIApplication) {
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
private extension CGRect {
    var center: CGPoint { CGPoint(x: midX, y: midY) }
}
