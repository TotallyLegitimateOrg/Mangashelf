/* SPDX-License-Identifier: GPL-3.0-or-later */

import { ContentRating, SourceIntents, type ExtensionInfo } from "@paperback/types";

export default {
  name: "Mangashelf",
  description: "Content extension for Mangashelf. Connect to your self-hosted Mangashelf server.",
  version: "1.0.1",
  icon: "icon.png",
  language: "en",
  contentRating: ContentRating.EVERYONE,
  capabilities: [
    SourceIntents.SETTINGS_FORM_PROVIDING,
    SourceIntents.DISCOVER_SECTION_PROVIDING,
    SourceIntents.SEARCH_RESULT_PROVIDING,
    SourceIntents.CHAPTER_PROVIDING,
    SourceIntents.MANAGED_COLLECTION_PROVIDING,
  ],
  badges: [],
  developers: [
    {
      name: "AJ",
    },
  ],
} satisfies ExtensionInfo;
