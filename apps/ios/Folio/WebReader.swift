import SwiftUI
import WebKit

enum WebSection {
  case commissions, knowledge
  var origin: URL { self == .commissions ? LibraryAPI.accounts : LibraryAPI.origin }
  var path: String { self == .commissions ? "ios-commissions" : "knowledge" }
}
struct ReaderWebPage: View {
  let section: WebSection
  @State private var target: Page?
  @State private var issue: String?
  @State private var download: Downloaded?
  private struct Downloaded: Identifiable {
    let id = UUID()
    let url: URL
  }
  var body: some View {
    ReaderWebContainer(
      section: section, world: { target = .world($0) }, file: { download = Downloaded(url: $0) },
      error: { issue = $0 }
    )
    .navigationTitle(section == .knowledge ? "作品知识库" : "我的约稿").navigationBarTitleDisplayMode(
      .inline
    )
    .navigationDestination(item: $target) { Destination(page: $0) }
    .sheet(item: $download) { item in ShareFile(url: item.url) }
    .alert("页面提示", isPresented: Binding(get: { issue != nil }, set: { if !$0 { issue = nil } })) {
      Button("好") { issue = nil }
    } message: {
      Text(issue ?? "")
    }
  }
}
private struct ShareFile: UIViewControllerRepresentable {
  let url: URL
  func makeUIViewController(context: Context) -> UIActivityViewController {
    UIActivityViewController(activityItems: [url], applicationActivities: nil)
  }
  func updateUIViewController(_ controller: UIActivityViewController, context: Context) {}
}
private struct ReaderWebContainer: UIViewRepresentable {
  let section: WebSection
  let world: (String) -> Void
  let file: (URL) -> Void
  let error: (String) -> Void
  func makeCoordinator() -> Coordinator { Coordinator(self) }
  func makeUIView(context: Context) -> WKWebView {
    let view = WKWebView()
    view.navigationDelegate = context.coordinator
    view.load(URLRequest(url: section.origin.appending(path: section.path)))
    return view
  }
  func updateUIView(_ view: WKWebView, context: Context) {}
  @MainActor final class Coordinator: NSObject, WKNavigationDelegate, WKDownloadDelegate {
    let parent: ReaderWebContainer
    private var purchasing = false
    var files: [ObjectIdentifier: URL] = [:]
    init(_ parent: ReaderWebContainer) { self.parent = parent }
    func webView(
      _ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
      decisionHandler: @escaping @MainActor @Sendable (WKNavigationActionPolicy) -> Void
    ) {
      let site = parent.section.origin
      if let url = action.request.url, url.scheme == "folio-wallet" {
        decisionHandler(.cancel)
        guard parent.section == .commissions, url.host == "topup",
          action.navigationType == .linkActivated, action.sourceFrame.isMainFrame,
          action.targetFrame?.isMainFrame == true, let page = action.sourceFrame.request.url,
          page.scheme == site.scheme, page.host == site.host, page.port == site.port,
          page.path == "/ios-commissions", let wallet = UUID(uuidString: String(url.path.dropFirst())),
          !purchasing else { return }
        purchasing = true
        Task {
          defer { purchasing = false }
          do {
            if let response = try await CommissionPurchase.buy(wallet: wallet) {
              webView.reload()
              parent.error(response)
            }
          } catch { parent.error(error.localizedDescription) }
        }
        return
      }
      guard let u = action.request.url, u.scheme == site.scheme,
        u.host == site.host, u.port == site.port
      else {
        decisionHandler(.cancel)
        return
      }
      let parts = u.path.split(separator: "/")
      if parent.section == .knowledge, parts.count == 2, parts[0] == "work" {
        decisionHandler(.cancel)
        parent.world(String(parts[1]))
        return
      }
      if parent.section == .commissions && parts.count == 4 && parts[0] == "reader-api"
        && parts[1] == "orders" && UUID(uuidString: String(parts[2])) != nil && parts[3] == "image"
      {
        decisionHandler(.download)
        return
      }
      let allowed =
        parent.section == .knowledge ? parts.first == "knowledge" : u.path == "/ios-commissions"
      decisionHandler(allowed ? .allow : .cancel)
    }
    func webView(
      _ webView: WKWebView, navigationAction: WKNavigationAction, didBecome download: WKDownload
    ) { download.delegate = self }
    func download(
      _ download: WKDownload, decideDestinationUsing response: URLResponse,
      suggestedFilename: String, completionHandler: @escaping @MainActor @Sendable (URL?) -> Void
    ) {
      guard (response as? HTTPURLResponse)?.statusCode == 200, response.mimeType == "image/png"
      else {
        completionHandler(nil)
        parent.error("原图暂不可用。")
        return
      }
      let dest = FileManager.default.temporaryDirectory.appending(
        path: "约稿-" + UUID().uuidString + ".png")
      files[ObjectIdentifier(download)] = dest
      completionHandler(dest)
    }
    func downloadDidFinish(_ download: WKDownload) {
      if let url = files.removeValue(forKey: ObjectIdentifier(download)) { parent.file(url) }
    }
    func download(_ download: WKDownload, didFailWithError error: Error, resumeData: Data?) {
      files.removeValue(forKey: ObjectIdentifier(download))
      parent.error("下载中断，请重试。")
    }
    func webView(
      _ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!,
      withError error: Error
    ) {
      if (error as NSError).code != NSURLErrorCancelled { parent.error(error.localizedDescription) }
    }
  }
}
