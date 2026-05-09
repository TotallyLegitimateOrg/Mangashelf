import { describe, expect, it } from "bun:test";

import { MangashelfExtension } from "./main.ts";

function createApplicationStub({ responseStatus = 200, responseBody, onRequest } = {}) {
  return {
    scheduleRequest: async (request) => {
      onRequest?.(request);
      const payload = JSON.stringify(responseBody ?? { manga: [] });
      return [{ status: responseStatus }, new TextEncoder().encode(payload).buffer];
    },
    arrayBufferToUTF8String: (buffer) => new TextDecoder().decode(buffer),
    getState: () => undefined,
    getSecureState: () => undefined,
    setState: () => undefined,
    setSecureState: () => undefined,
    Selector: () => "selector",
  };
}

function createExtension(application) {
  globalThis.Application = application;
  const extension = new MangashelfExtension();
  extension.settingsForm.setPersistedValues({
    serverUrl: "http://localhost:8080/",
    apiKey: "secret",
  });
  return extension;
}

describe("MangashelfExtension search", () => {
  it("serializes advanced filters and sorting into the API request", async () => {
    const requests = [];
    const extension = createExtension(
      createApplicationStub({
        responseBody: {
          manga: [
            {
              id: "m1",
              primaryTitle: "Alpha",
              secondaryTitles: [],
              synopsis: "",
              thumbnailUrl: "thumb",
              bannerUrl: "",
              contentRating: "MATURE",
              status: "Ongoing",
              artist: "",
              author: "Author A",
              rating: 8.5,
              shareUrl: "",
              artworkUrls: [],
              tagGroups: [],
              additionalInfo: [],
              chapterCount: 12,
              createdAt: "2026-01-01T00:00:00Z",
              updatedAt: "2026-01-02T00:00:00Z",
            },
          ],
        },
        onRequest: (request) => requests.push(request),
      }),
    );

    const result = await extension.getSearchResults(
      {
        title: "alpha",
        metadata: [
          { id: "contentRating", value: { MATURE: "included", SAFE: "excluded" } },
          { id: "status", value: { Ongoing: "included" } },
          { id: "tags", value: "Genres:Action, Comedy" },
          { id: "minRating", value: "8" },
          { id: "maxRating", value: "9.5" },
        ],
      },
      undefined,
      { id: "rating_desc", label: "Rating High-Low" },
    );

    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({
      url: "http://localhost:8080/api/manga?q=alpha&contentRating=MATURE&status=Ongoing&tag=Genres%3AAction&tag=Comedy&minRating=8&maxRating=9.5&sort=rating_desc",
      method: "GET",
      headers: {
        Authorization: "Bearer secret",
        "Content-Type": "application/json",
      },
    });
    expect(result.items).toEqual([
      {
        mangaId: "m1",
        title: "Alpha",
        subtitle: "Author A",
        imageUrl: "thumb",
        metadata: {
          chapterCount: 12,
          status: "Ongoing",
          rating: 8.5,
        },
        contentRating: "MATURE",
      },
    ]);
  });

  it("throws a visible error when the search request fails", async () => {
    const extension = createExtension(createApplicationStub({ responseStatus: 401 }));

    await expect(
      extension.getSearchResults({
        title: "alpha",
        metadata: undefined,
      }),
    ).rejects.toThrow("Search failed: API request failed: 401");
  });

  it("computes a chapter title when the API returns a blank one", async () => {
    const extension = createExtension(
      createApplicationStub({
        responseBody: {
          chapters: [
            {
              id: "c1",
              mangaId: "m1",
              langCode: "EN",
              chapNum: 19,
              title: "",
              version: "",
              volume: null,
              publishDate: null,
              creationDate: null,
              sortingIndex: null,
              additionalInfo: [],
              pageCount: 12,
              lastUpdated: "2026-01-02T00:00:00Z",
            },
          ],
        },
      }),
    );

    const chapters = await extension.getChapters({ mangaId: "m1" });

    expect(chapters).toEqual([
      expect.objectContaining({
        chapterId: "c1",
        chapNum: 19,
        title: "Chapter 19",
      }),
    ]);
  });

  it("computes a discover chapter subtitle when the API returns a blank one", async () => {
    const extension = createExtension(
      createApplicationStub({
        responseBody: {
          sections: [
            {
              id: "s1",
              title: "Latest chapters",
              subtitle: "",
              type: "chapterUpdates",
              sortOrder: 0,
              items: [
                {
                  id: "item-1",
                  type: "chapterUpdatesCarouselItem",
                  mangaId: "m1",
                  chapterId: "c1",
                  chapNum: 19,
                  imageUrl: "thumb",
                  title: "Series",
                  subtitle: "",
                  publishDate: null,
                  contentRating: null,
                  metadata: null,
                },
              ],
            },
          ],
        },
      }),
    );

    const sections = await extension.getDiscoverSections();
    const items = await extension.getDiscoverSectionItems(sections[0], undefined);

    expect(items.items).toEqual([
      expect.objectContaining({
        type: "chapterUpdatesCarouselItem",
        chapterId: "c1",
        subtitle: "Chapter 19",
      }),
    ]);
  });
});
