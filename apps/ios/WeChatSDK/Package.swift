// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "FolioWeChatSDK",
    platforms: [.iOS(.v17)],
    products: [.library(name: "WechatOpenSDK", targets: ["WechatOpenSDK"])],
    targets: [
        .binaryTarget(name: "WechatOpenSDK",
                      url: "https://dldir1.qq.com/WechatWebDev/opensdk/XCFramework/OpenSDK2.1.0.zip",
                      checksum: "1f46ce001d14c9aaad598d6ebfd50ddb4113afa31e13a3fbf2366f0488ff1310")
    ]
)
