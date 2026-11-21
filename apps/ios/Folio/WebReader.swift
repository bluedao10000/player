import SwiftUI
import WebKit

struct ReaderWebPage: View {
  @State private var target: Page?
  @State private var issue: String?
  var body: some View {
    ReaderWebContainer(
      world: { target = .world($0) },
      error: { issue = $0 }
    )
    .navigationTitle("作品知识库").navigationBarTitleDisplayMode(
      .inline
    )
    .navigationDestination(item: $target) { Destination(page: $0) }
    .alert("页面提示", isPresented: Binding(get: { issue != nil }, set: { if !$0 { issue = nil } })) {
      Button("好") { issue = nil }
    } message: {
      Text(issue ?? "")
    }
  }
}
private struct ReaderWebContainer: UIViewRepresentable {
  let world: (String) -> Void
  let error: (String) -> Void
  func makeCoordinator() -> Coordinator { Coordinator(self) }
  func makeUIView(context: Context) -> WKWebView {
    let view = WKWebView()
    view.navigationDelegate = context.coordinator
    view.load(URLRequest(url: LibraryAPI.origin.appending(path: "knowledge")))
    return view
  }
  func updateUIView(_ view: WKWebView, context: Context) {}
  @MainActor final class Coordinator: NSObject, WKNavigationDelegate {
    let parent: ReaderWebContainer
    init(_ parent: ReaderWebContainer) { self.parent = parent }
    func webView(
      _ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
      decisionHandler: @escaping @MainActor @Sendable (WKNavigationActionPolicy) -> Void
    ) {
      let site = LibraryAPI.origin
      guard let u = action.request.url, u.scheme == site.scheme,
        u.host == site.host, u.port == site.port
      else {
        decisionHandler(.cancel)
        return
      }
      let parts = u.path.split(separator: "/")
      if parts.count == 2, parts[0] == "work" {
        decisionHandler(.cancel)
        parent.world(String(parts[1]))
        return
      }
      decisionHandler(parts.first == "knowledge" ? .allow : .cancel)
    }
    func webView(
      _ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!,
      withError error: Error
    ) {
      if (error as NSError).code != NSURLErrorCancelled { parent.error(error.localizedDescription) }
    }
  }
}
