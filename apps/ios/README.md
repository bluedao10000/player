# Folio 时光 iOS

用 Swift 6、SwiftUI、URLSession 和 XcodeGen 构建。最低支持 iOS 17。读者可以搜索作品、按作品浏览人物与故事，在场景页回到章节原段落。访客书签保存在本机。登录后可选择导入，书签和段落位置与 H5 共享。角色和章节可以分享同一内容的 H5 原链接。

## 启动与签名

从本目录执行以下命令。选择 iPhone 模拟器，运行 AnimeApp-Dev。本地 H5 使用仓库的开发启动方式，目录请求依次经过 H5、Gin 和 PostgreSQL。

~~~sh
xcodegen generate
open AnimeApp.xcodeproj
~~~

| Scheme | Bundle ID | 默认内容站 |
| --- | --- | --- |
| AnimeApp-Dev | com.kmszz.starWiki.dev | http://127.0.0.1:4310 |
| AnimeApp-Test | com.kmszz.starWiki.test | https://www.codeiy.com |
| AnimeApp-Sandbox | com.kmszz.starWiki | 正式站内容，独立沙盒账户与支付 |
| AnimeApp-Prod | com.kmszz.starWiki | https://www.codeiy.com |

可在 Xcode 的 Edit Scheme → Run → Arguments → Environment Variables 设置 FOLIO_SITE_URL，指向包含本版只读接口和图片的站点。模拟器可以使用 Mac 的回环地址；真机联调使用设备可访问的 HTTPS 地址。

正式包沿用旧应用的标识和 Apple 团队自动签名。选择 AnimeApp-Prod 和 Any iOS Device，执行 Product → Archive，在 Organizer 选择对应归档分发。版本号与构建号在 project.yml 中维护。首次真机分发前核对目标 App Store Connect 记录的最新上传版本和构建号。工程使用钥匙串中的签名身份。

## 真机验收

连接已启用开发者模式的 iPhone，在 Xcode 选择 AnimeApp-Test 和该设备，点击 Run。测试版使用独立 Bundle ID，默认读取正式站。如使用已导出的开发签名 IPA，可在 Xcode 的 Window → Devices and Simulators 中选择描述文件已登记的手机，通过 Installed Apps 下的加号安装。

## 阅读接口

正式站的[作品目录](https://www.codeiy.com/ios-data/v1/discovery)可以直接在设备上打开。读取支持 GET 和 HEAD，其他方法返回 405。

App 从内容站读取下表的 GET 路径，图片沿用站点的 /art/ 地址。服务端按字段返回发布内容。人物、章节和场景 slug 以所属作品为作用域。场景的 anchor 与章节 blocks 中的 anchors 对应。

| 路径（以 /ios-data/v1 开头） | 内容 |
| --- | --- |
| /discovery | 作品、角色图鉴与故事目录 |
| /works/{work} | 作品简介、故事线、人物、场景与世界观 |
| /works/{work}/characters/{slug} | 人物介绍和出场章节 |
| /works/{work}/chapters/{slug} | 章节正文、段落锚点、入选插画与下一章 |
| /works/{work}/scenes/{slug} | 场景正文、在场人物与章节回跳 |

## 验收

在 Xcode 中选择 AnimeApp-Dev 并按 ⌘U。UI 测试使用本地知识库中已发布的《潮汐邮局》，从阿澪的出场章节打开《末班渡船》，收藏后查看「旧渡口核对名册」，回跳正文第二段，重启后从「我的」恢复。命令行可将 destination 替换为本机可用模拟器的名称。

~~~sh
xcodebuild -project AnimeApp.xcodeproj -scheme AnimeApp-Dev -destination 'platform=iOS Simulator,name=Folio 时光 iPhone 17' -parallel-testing-enabled NO test
xcodebuild -project AnimeApp.xcodeproj -scheme AnimeApp-Prod -destination 'generic/platform=iOS' -archivePath build/Folio.xcarchive archive
~~~

## 读者账户与购买

| 入口 | App 行为 | 数据归属 |
| --- | --- | --- |
| 我的 → 微信登录 | 通过移动应用 OpenSDK 授权，Gin 向微信兑换授权码 | 按 AppID 和 OpenID 识别，已验证的 UnionID 用于关联网页账户 |
| 我的 → 通过 Apple 登录 | 系统授权面板，服务端核对 Apple 签名、受众、有效期和一次性 nonce | Apple subject 绑定读者 UUID |
| 我的 → 邮箱验证码 | 复用网页发码、绑定及读者会话 | 登录凭据保存在设备钥匙串 |
| 首次登录 → 合并或跳过 | 导入本机书架及段落或保留访客记录，登录已有账户可绑定其他方式 | reader_bookmarks、reader_positions |
| 我的 → 会员 | StoreKit 2 拉取当前商店实际价格，可购买、恢复、重试未完成交易、查看 App Store 购买记录 | Gin 验证 Apple JWS 和 appAccountToken 后更新 PostgreSQL，H5 读取同一会员期和积分 |
| 发现 → 作品知识库 | 查询已收录的作品，打开已发布的作品世界 | 线上知识库 |
| 章节 → 进入剧情 | 展示 H5 已提供的互动分支及选项，在本机记住选择 | 沿用对应故事的剧本 |

### 微信移动应用

使用微信开放平台登记到正式 Bundle ID 的移动应用。在项目根目录的本地 .env 写入 WECHAT_IOS_APP_ID 与 WECHAT_IOS_APP_SECRET，并重新创建 API 容器。将公开字段分别写入 apps/ios/Config/WeChat.local.xcconfig 的 WECHAT_IOS_APP_ID、WECHAT_IOS_LINK_HOST 和 WECHAT_IOS_UNIVERSAL_LINK。xcconfig 中的 https 地址按下面的形式转义双斜线。用已登记域名和完整路径替换示例。

~~~xcconfig
WECHAT_IOS_APP_ID = wx应用标识
WECHAT_IOS_LINK_HOST = 已登记域名
WECHAT_IOS_UNIVERSAL_LINK = https:/$()/已登记域名/已登记路径/
~~~

Universal Link 域名需要提供 apple-app-site-association，包含 YOUR_TEAM_ID.com.kmszz.starWiki 和登记的路径。使用 AnimeApp-Prod 签名安装，检查授权后能从微信返回。若已有网页读者账户，先用原登录方式进入并绑定移动微信或 Apple，可延续原书架与会员。

### 新建 App Store 商品

在 com.kmszz.starWiki 对应的 App Store Connect 记录下配置两个非续期订阅，期限由服务端从购买时间计算。中国区目标价格沿用 Web，其他商店在 App 中显示 StoreKit 的本地化价格。

| Product ID | 期限 | 中国区目标价 | 本期积分 |
| --- | --- | --- | --- |
| com.kmszz.starWiki.plus.30days | 30 天 | ¥20 | 125 |
| com.kmszz.starWiki.pro.30days | 30 天 | ¥99 | 625 |


使用 App Store Server Notifications V2，将生产通知指向 https://www.codeiy.com/ios-account/native/store/notifications 。沙盒需单独的可访问部署、数据库和通知地址，建议使用 ios-sandbox.codeiy.com 单独承接沙盒。deploy/compose.ios-sandbox.yaml 建立独立 PostgreSQL 容器、卷、数据库、网络和 API，API 绑定本机 14421。设置独立的 IOS_SANDBOX_DB_PASSWORD，使用第二个 H5 实例并将 KNOWLEDGE_API_URL 指向 http://127.0.0.1:14421 ，沙盒域名和沙盒通知地址路由到该 H5。Apple 沙盒测试使用 AnimeApp-Sandbox，保留 App Store 商品所属的正式 Bundle ID，通过独立钥匙串条目访问沙盒账户；目录和图像仍从正式内容站读取。在沙盒新建测试读者账户。生产默认只接受 Production。交易验证采用 Apple Root CA G3，检查签名链、应用、环境、商品类型和账户。设备仅在服务端入账成功后结束交易。恢复使用原始购买时间，退款和撤销退款使用已签名事件的先后顺序。

### 本地验收范围

根目录运行 npm run check，并运行开发规范中的隔离 PostgreSQL 测试。测试环境保留数据库连接参数，取消本地 MATCH_AI_PROVIDER 与 TYPESAFE_API_KEY 的覆盖可使匹配测试使用其内置服务。使用 AnimeApp-Dev 连接本机 H5 和 Gin，执行 ⌘U。StoreTests 覆盖两档会员购买，检查交易绑定的读者 UUID。服务端验签、重投、退款和权益期限另有 Go / PostgreSQL 测试。本地 StoreKit 测试收据与 App Store 沙盒签名相互独立。最终真机验收需要在已登记的微信客户端、真实 Apple 账号和已配置商品的 Apple 沙盒中完成。
