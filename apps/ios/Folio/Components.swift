import SwiftUI

let ink = Color(red: 0.055, green: 0.055, blue: 0.086)
let violet = Color(red: 0.82, green: 0.76, blue: 1)
let mist = Color(red: 0.66, green: 0.84, blue: 0.85)

struct Artwork: View {
    let path: String?
    var body: some View {
        GeometryReader { geo in
            ZStack {
                LinearGradient(colors: [Color.indigo.opacity(0.65), ink], startPoint: .topTrailing, endPoint: .bottomLeading)
                AsyncImage(url: LibraryAPI.image(path)) { phase in
                    if let image = phase.image {
                        image.resizable().scaledToFill()
                    } else {
                        Image(systemName: "book.pages").font(.largeTitle).foregroundStyle(violet.opacity(0.5))
                    }
                }
            }.frame(width: geo.size.width, height: geo.size.height).clipped()
        }.accessibilityHidden(true)
    }
}

struct Cover: View {
    let path: String?
    let title, label: String
    var height: CGFloat = 300
    var body: some View {
        ZStack(alignment: .bottomLeading) {
            Artwork(path: path)
            LinearGradient(colors: [.clear, .black.opacity(0.15), .black.opacity(0.92)], startPoint: .top, endPoint: .bottom)
            VStack(alignment: .leading, spacing: 7) {
                Text(label).font(.subheadline.weight(.medium)).foregroundStyle(mist)
                Text(title).font(.system(.largeTitle, design: .serif, weight: .bold))
            }.padding(20).foregroundStyle(.white).multilineTextAlignment(.leading)
        }.frame(minHeight: height).frame(height: height).clipShape(.rect(cornerRadius: 24))
    }
}

struct EntryRow: View {
    let title, subtitle: String
    var image: String?
    var body: some View {
        HStack(spacing: 14) {
            if let image {
                Artwork(path: image).frame(width: 68, height: 70).clipShape(.rect(cornerRadius: 12))
            }
            VStack(alignment: .leading, spacing: 5) {
                Text(title).font(.headline).foregroundStyle(.primary)
                if !subtitle.isEmpty { Text(subtitle).font(.subheadline).foregroundStyle(.secondary).lineLimit(2) }
            }
            Spacer(minLength: 4)
            Image(systemName: "chevron.right").font(.caption).foregroundStyle(violet)
        }.padding(.vertical, 6).contentShape(Rectangle()).multilineTextAlignment(.leading)
    }
}

struct ReaderSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title).font(.system(.title2, design: .serif, weight: .bold)).accessibilityAddTraits(.isHeader)
            content
        }.frame(maxWidth: .infinity, alignment: .leading).padding(.top, 12)
    }
}

struct RemotePage<Value: Decodable, Content: View>: View {
    let path: String
    @ViewBuilder let content: (Value) -> Content
    @State private var value: Value?
    @State private var failed = false
    @State private var attempt = 0
    var body: some View {
        Group {
            if let value { content(value) }
            else if failed {
                ContentUnavailableView {
                    Label("暂时无法载入", systemImage: "wifi.exclamationmark")
                } description: {
                    Text("请检查网络连接，然后重试。")
                } actions: {
                    Button("重新载入") { attempt += 1 }
                }
            } else { ProgressView("正在载入") }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(ink)
        .task(id: "\(path)#\(attempt)") {
            failed = false
            do { value = try await LibraryAPI.load(path) }
            catch { if !Task.isCancelled { failed = true } }
        }
    }
}
