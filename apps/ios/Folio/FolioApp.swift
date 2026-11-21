import SwiftUI

@main
struct FolioApp: App {
  @State private var shelf = Shelf()
  @State private var store = StoreMembership()
  @Environment(\.scenePhase) private var phase
  var body: some SwiftUI.Scene {
    WindowGroup {
      TabView {
        ForEach(Directory.allCases, id: \.self) { kind in
          NavigationStack {
            RemotePage(path: "discovery") { (catalog: Catalog) in
              DirectoryView(kind: kind, works: catalog.items)
            }
            .navigationTitle(kind.title)
            .navigationDestination(for: Page.self) { Destination(page: $0) }
          }
          .tabItem { Label(kind.title, systemImage: kind.icon) }
        }
        NavigationStack {
          MyShelf()
            .navigationDestination(for: Page.self) { Destination(page: $0) }
        }.tabItem { Label("我的", systemImage: "bookmark") }
      }.environment(shelf).environment(store).tint(violet).preferredColorScheme(.dark)
        .task(id: phase) {
          if phase == .active && shelf.token != nil {
            do {
              try await shelf.flush()
              try await shelf.refresh()
            } catch { shelf.issue = error.localizedDescription }
          }
        }
        .task(id: shelf.user?.id) { await store.observe(shelf) }
        .onOpenURL { WechatLogin.shared.handle($0) }
        .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { WechatLogin.shared.handle($0) }
        .alert(
          "读者账户",
          isPresented: Binding(get: { shelf.issue != nil }, set: { if !$0 { shelf.issue = nil } })
        ) {
          Button("好") { shelf.issue = nil }
        } message: {
          Text(shelf.issue ?? "")
        }
    }
  }
}

enum Directory: CaseIterable {
  case discover, cast, chapters
  var title: String {
    switch self {
    case .discover: "发现"
    case .cast: "角色"
    case .chapters: "故事"
    }
  }
  var icon: String {
    switch self {
    case .discover: "safari"
    case .cast: "person.2"
    case .chapters: "book.pages"
    }
  }
}

struct DirectoryView: View {
  let kind: Directory
  let works: [Work]
  @State private var query = ""
  @State private var workFilter: String?
  private var filtered: [Work] {
    works.filter { w in
      (workFilter == nil || w.slug == workFilter) && (kind != .discover || matches(w.title))
    }
  }
  private func matches(_ text: String) -> Bool {
    let term = query.trimmingCharacters(in: .whitespacesAndNewlines)
    return term.isEmpty || text.localizedCaseInsensitiveContains(term)
  }
  private var featured: Work? {
    works.filter { $0.image != nil }.max {
      String($0.edition.prefix(4)).filter(\.isNumber)
        < String($1.edition.prefix(4)).filter(\.isNumber)
    }
  }
  var body: some View {
    ScrollView {
      LazyVStack(alignment: .leading, spacing: 18) {
        if kind == .discover {
          if query.isEmpty, let lead = featured {
            NavigationLink(value: Page.world(lead.slug)) {
              Cover(path: lead.image, title: lead.title, label: lead.edition)
            }
          }
          NavigationLink {
            ReaderWebPage()
          } label: {
            Label("作品知识库", systemImage: "books.vertical").foregroundStyle(mist)
          }
          ReaderSection(title: "作品世界") {
            if filtered.isEmpty { ContentUnavailableView.search(text: query) }
            LazyVGrid(
              columns: [GridItem(.adaptive(minimum: 146), spacing: 14)], alignment: .leading,
              spacing: 20
            ) {
              ForEach(filtered) { work in
                NavigationLink(value: Page.world(work.slug)) {
                  VStack(alignment: .leading, spacing: 6) {
                    Artwork(path: work.image).aspectRatio(1.42, contentMode: .fit).clipShape(
                      .rect(cornerRadius: 14))
                    Text(work.title).font(.headline).lineLimit(2)
                    Text(work.edition).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                  }.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityIdentifier("world-" + work.slug)
              }
            }
          }
        } else {
          Picker("按作品筛选", selection: $workFilter) {
            Text("全部作品").tag(nil as String?)
            ForEach(works) { Text($0.title).tag(Optional($0.slug)) }
          }.pickerStyle(.menu).tint(mist)
          let groups = filtered.map { w in
            let entries = kind == .cast ? w.characters : w.chapters
            return (w, (entries ?? []).filter { matches($0.title) || matches(w.title) })
          }.filter { !$0.1.isEmpty }
          if groups.isEmpty { ContentUnavailableView.search(text: query) }
          ForEach(groups, id: \.0.slug) { w, entries in
            ReaderSection(title: w.title) {
              ForEach(entries) { item in
                NavigationLink(
                  value: kind == .cast
                    ? Page.person(w.slug, item.slug) : .chapter(w.slug, item.slug)
                ) {
                  EntryRow(title: item.title, subtitle: item.subtitle, image: item.image)
                }
              }
            }
          }
        }
      }.padding(.horizontal, 18).padding(.bottom, 26)
    }
    .searchable(text: $query, prompt: kind == .discover ? "搜索作品" : "搜索\(kind.title)或作品")
    .buttonStyle(.plain)
  }
}

struct MyShelf: View {
  @Environment(Shelf.self) private var shelf
  var body: some View {
    ScrollView {
      VStack(alignment: .leading, spacing: 26) {
        ReaderAccountPanel()
        if let last = shelf.records.recent.first {
          ReaderSection(title: "继续阅读") {
            NavigationLink(value: last.page) {
              EntryRow(title: last.title, subtitle: last.worldTitle)
            }.accessibilityIdentifier("continue-reading")
          }
        }
        ReaderSection(title: "我的收藏") {
          if shelf.records.saved.isEmpty {
            Text("阅读时轻点书签，就能把章节收藏到这台设备。").foregroundStyle(.secondary)
          }
          ForEach(shelf.records.saved) { item in
            let position = shelf.records.recent.first { $0.id == item.id }
            NavigationLink(value: (position ?? item).page) {
              EntryRow(title: item.title, subtitle: item.worldTitle)
            }
          }
        }
        if shelf.records.recent.count > 1 {
          ReaderSection(title: "最近读过") {
            ForEach(shelf.records.recent.dropFirst()) { item in
              NavigationLink(value: item.page) {
                EntryRow(title: item.title, subtitle: item.worldTitle)
              }
            }
          }
        }
      }.padding(20)
    }.navigationTitle("我的").frame(maxWidth: .infinity, maxHeight: .infinity)
      .background(ink).buttonStyle(.plain)
  }
}
