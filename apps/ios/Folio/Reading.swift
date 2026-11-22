import SwiftUI

struct Destination: View {
  let page: Page
  var body: some View {
    switch page {
    case .world(let work):
      RemotePage(path: "works/" + work) { (w: World) in
        DetailScroll {
          Cover(path: w.work.image, title: w.work.title, label: w.work.edition)
          Prose(text: w.premise)
          Links(title: "角色", items: w.characters) { .person(work, $0) }
          ForEach(Array(w.stories.enumerated()), id: \.offset) { _, story in
            Links(title: story.title, items: story.chapters) { .chapter(work, $0) }
          }
          Links(title: "场景", items: w.scenes) { .scene(work, $0) }
          if let setting = w.setting, !setting.isEmpty {
            ReaderSection(title: "世界观") { Prose(text: setting) }
          }
        }.navigationTitle(w.work.title)
      }
    case .person(let work, let slug):
      RemotePage(path: "works/\(work)/characters/\(slug)") { (p: Person) in
        DetailScroll {
          Cover(path: p.person.image, title: p.person.title, label: p.person.subtitle, height: 360)
          if let intro = p.intro { Prose(text: intro) }
          Links(title: "出场章节", items: p.chapters) { .chapter(work, $0) }
          Links(title: "相关场景", items: p.scenes) { .scene(work, $0) }
        }.navigationTitle(p.person.title)
          .toolbar { WorldMenu(work: work, href: p.person.href) }
      }
    case .scene(let work, let slug):
      RemotePage(path: "works/\(work)/scenes/\(slug)") { (s: Scene) in
        DetailScroll {
          Cover(
            path: s.scene.image ?? s.chapter.image, title: s.scene.title, label: s.scene.subtitle)
          Prose(text: s.text)
          NavigationLink(value: Page.chapter(work, s.chapter.slug, s.anchor)) {
            Label("回到章节", systemImage: "text.book.closed").frame(maxWidth: .infinity)
          }.buttonStyle(.borderedProminent).foregroundStyle(ink)
          Links(title: "本幕角色", items: s.cast) { .person(work, $0) }
        }.navigationTitle(s.scene.title)
          .toolbar { WorldMenu(work: work, href: s.scene.href) }
      }
    case .chapter(let work, let slug, let anchor):
      RemotePage(path: "works/\(work)/chapters/\(slug)") { (c: Chapter) in
        Reader(work: work, data: c, initialAnchor: anchor)
      }
    }
  }
}

struct DetailScroll<Content: View>: View {
  @ViewBuilder let content: Content
  var body: some View {
    ScrollView {
      VStack(alignment: .leading, spacing: 18) { content }
        .frame(maxWidth: 760).padding(18).frame(maxWidth: .infinity)
    }.buttonStyle(.plain).navigationBarTitleDisplayMode(.inline)
  }
}

struct Links: View {
  let title: String
  let items: [Entry]
  let page: (String) -> Page
  var body: some View {
    if !items.isEmpty {
      ReaderSection(title: title) {
        ForEach(items) { entry in
          NavigationLink(value: page(entry.slug)) {
            EntryRow(title: entry.title, subtitle: entry.subtitle, image: entry.image)
          }.accessibilityIdentifier("entry-" + entry.slug)
        }
      }
    }
  }
}

struct Prose: View {
  let text: String
  var body: some View {
    Text(text).font(.system(.body, design: .serif)).lineSpacing(8)
      .frame(maxWidth: .infinity, alignment: .leading).textSelection(.enabled)
  }
}

private struct ParagraphOffsets: PreferenceKey {
  static let defaultValue: [String: CGFloat] = [:]
  static func reduce(value: inout [String: CGFloat], nextValue: () -> [String: CGFloat]) {
    value.merge(nextValue(), uniquingKeysWith: { _, new in new })
  }
}

struct Reader: View {
  let work: String
  let data: Chapter
  let initialAnchor: String?
  @Environment(Shelf.self) private var shelf
  @State private var tracking = false
  @State private var current: String?

  private var item: SavedChapter {
    SavedChapter(
      work: work, slug: data.chapter.slug, title: data.chapter.title, worldTitle: data.worldTitle,
      anchor: current,
      href: data.chapter.href,
      percent: 100 * (data.blocks.firstIndex { $0.anchor == current } ?? 0)
        / max(data.blocks.count - 1, 1),
      excerpt: String((data.blocks.first { $0.anchor == current }?.text ?? "").prefix(160)))
  }
  private var saved: Bool { shelf.records.saved.contains { $0.id == item.id } }

  var body: some View {
    GeometryReader { viewport in
      ScrollViewReader { proxy in
        ScrollView {
          VStack(alignment: .leading, spacing: 22) {
            Cover(
              path: data.chapter.image, title: data.chapter.title, label: data.chapter.subtitle,
              height: 240)
            if let intro = data.chapter.intro { Text(intro).foregroundStyle(mist).font(.headline) }
            ForEach(data.blocks) { block in
              Prose(text: block.text)
                .background(
                  GeometryReader { geo in
                    Color.clear.preference(
                      key: ParagraphOffsets.self,
                      value: [block.key: geo.frame(in: .named("reading")).minY])
                  }
                )
                .id(block.key)
                .accessibilityIdentifier("passage-" + block.key)
              ForEach(data.illustrations.filter { $0.block == block.key }, id: \.image) { pic in
                Artwork(path: pic.image).aspectRatio(1.55, contentMode: .fit).clipShape(
                  .rect(cornerRadius: 15)
                ).accessibilityLabel(pic.name)
              }
            }
            if let branch = data.play {
              NavigationLink {
                StoryBranch(branch: branch, slug: data.chapter.slug)
              } label: {
                Label("进入剧情", systemImage: "sparkles").frame(maxWidth: .infinity)
              }.buttonStyle(.borderedProminent).foregroundStyle(ink)
            }
            Links(title: "本章场景", items: data.scenes) { .scene(work, $0) }
            if let next = data.next {
              NavigationLink(value: Page.chapter(work, next.slug)) {
                EntryRow(
                  title: next.title,
                  subtitle: next.subtitle.isEmpty ? "下一篇" : "下一篇 · " + next.subtitle, image: next.image)
              }
            }
          }.frame(maxWidth: 760).padding(18).padding(.bottom, viewport.size.height * 0.6).frame(
            maxWidth: .infinity)
        }
        .coordinateSpace(name: "reading")
        .onPreferenceChange(ParagraphOffsets.self) { positions in
          if tracking { recordPosition(positions) }
        }
        .task {
          let target =
            current ?? initialAnchor
            ?? shelf.records.recent.first(where: { $0.id == item.id })?.anchor
          if let target,
            let block = data.blocks.first(where: {
              $0.anchor == target || $0.anchors.contains(target)
            })
          {
            current = block.anchor
            await Task.yield()
            proxy.scrollTo(block.key, anchor: .top)
          } else {
            current = data.blocks.first?.anchor
          }
          shelf.remember(item)
          tracking = true
        }
        .onDisappear { tracking = false }
      }
    }
    .navigationTitle(data.chapter.title).navigationBarTitleDisplayMode(.inline)
    .buttonStyle(.plain)
    .toolbar {
      ToolbarItemGroup(placement: .topBarTrailing) {
        Button {
          shelf.toggle(item)
        } label: {
          Image(systemName: saved ? "bookmark.fill" : "bookmark")
        }.accessibilityLabel(saved ? "移出书架" : "加入书架")
        WorldMenu(work: work, href: data.chapter.href)
      }
    }
  }

  private func recordPosition(_ positions: [String: CGFloat]) {
    guard
      let block = data.blocks.last(where: { (positions[$0.key] ?? .infinity) <= 45 })
        ?? data.blocks.first,
      current != block.anchor
    else { return }
    current = block.anchor
    shelf.remember(item)
  }
}

private struct WorldMenu: View {
  let work, href: String
  var body: some View {
    Menu {
      NavigationLink(value: Page.world(work)) { Label("进入作品世界", systemImage: "books.vertical") }
      if href.hasPrefix("/"), !href.hasPrefix("//"),
        let link = URL(string: "https://www.codeiy.com" + href)
      {
        ShareLink(item: link) { Label("分享网页链接", systemImage: "square.and.arrow.up") }
      }
    } label: {
      Image(systemName: "ellipsis.circle")
    }
    .accessibilityLabel("作品与分享")
  }
}

private struct StoryBranch: View {
  let branch: Branch
  @AppStorage private var selected: Int
  init(branch: Branch, slug: String) {
    self.branch = branch
    _selected = AppStorage(wrappedValue: -1, "folio.branch." + slug)
  }
  var body: some View {
    DetailScroll {
      Artwork(path: branch.image).aspectRatio(1.05, contentMode: .fit).clipShape(
        .rect(cornerRadius: 20))
      Prose(text: branch.scene)
      if branch.options.indices.contains(selected), branch.replies.indices.contains(selected) {
        Text(branch.options[selected]).foregroundStyle(mist)
        Prose(text: branch.replies[selected])
        Button("尝试另一种选择") { selected = -1 }.buttonStyle(.bordered)
      } else {
        ForEach(branch.options.indices, id: \.self) { i in
          Button(branch.options[i]) { selected = i }.buttonStyle(.bordered)
        }
      }
    }.navigationTitle("剧情互动")
  }
}
