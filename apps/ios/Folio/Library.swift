import Foundation
import SwiftUI

struct Work: Decodable, Identifiable {
    let slug, title, edition, intro, href: String
    let image: String?
    let characters, chapters: [Entry]?
    var id: String { slug }
}

struct Entry: Decodable, Identifiable {
    let slug, title, subtitle, href: String
    let intro, image: String?
    var id: String { slug }
}

struct Catalog: Decodable { let items: [Work] }
struct Storyline: Decodable { let title: String; let chapters: [Entry] }
struct World: Decodable {
    let work: Work
    let premise: String
    let setting: String?
    let stories: [Storyline]
    let characters, scenes: [Entry]
}
struct Person: Decodable {
    let person: Entry
    let intro: String?
    let chapters, scenes: [Entry]
}
struct Scene: Decodable {
    let scene, chapter: Entry
    let text: String
    let anchor: String?
    let cast: [Entry]
}
struct Passage: Decodable, Identifiable {
    let key, text: String
    let anchors: [String]
    var id: String { key }
    var anchor: String { anchors.first ?? "reading-" + key }
}
struct Illustration: Decodable { let block, image, name: String }
struct Branch: Decodable {let scene, image: String; let options, replies: [String]}
struct Chapter: Decodable {
    let play: Branch?
    let worldTitle: String
    let chapter: Entry
    let blocks: [Passage]
    let illustrations: [Illustration]
    let scenes: [Entry]
    let next: Entry?
}

enum LibraryAPI {
    static let origin: URL = {
        #if FOLIO_DEV
        let fallback = "http://127.0.0.1:4310"
        #else
        let fallback = "https://www.codeiy.com"
        #endif
        if let override = ProcessInfo.processInfo.environment["FOLIO_SITE_URL"],
           let url = URL(string: override), url.host != nil, ["https", "http"].contains(url.scheme) {
            return url
        }
        return URL(string: fallback)!
    }()

    static let accounts: URL = {
        #if FOLIO_SANDBOX
        return URL(string: "https://ios-sandbox.codeiy.com")!
        #else
        return origin
        #endif
    }()

    static func image(_ path: String?) -> URL? {
        guard let path, path.hasPrefix("/art/") else { return nil }
        return origin.appending(path: String(path.dropFirst()))
    }

    static func load<T: Decodable>(_ path: String, as: T.Type = T.self) async throws -> T {
        let url = origin.appending(path: "ios-data/v1/" + path)
        let (data, response) = try await URLSession.shared.data(from: url)
        guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw URLError(.badServerResponse) }
        return try JSONDecoder().decode(T.self, from: data)
    }
}

enum Page: Hashable {
    case world(String)
    case person(String, String)
    case chapter(String, String, String? = nil)
    case scene(String, String)
}

struct SavedChapter: Codable, Identifiable {
    let work, slug, title, worldTitle: String
    var anchor: String?
    var href: String? = nil
    var percent: Int? = nil
    var excerpt: String? = nil
    var recordedAt: Int64? = nil
    var canonical: String { href ?? (work == "tidepost" ? "/story/" + slug : "/work/" + work + "/chapter/" + slug) }
    var id: String { work + "/" + slug }
    var page: Page { .chapter(work, slug, anchor) }
}
