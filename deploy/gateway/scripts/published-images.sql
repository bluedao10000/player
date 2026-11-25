-- Export the selected references from an existing reader database.
SELECT DISTINCT reference FROM (
    SELECT card->>'hero_asset_key' AS reference FROM published_worlds
    UNION ALL SELECT card->>'portrait' FROM published_characters
    UNION ALL SELECT card->>'portrait' FROM extension_people
    UNION ALL SELECT card->>'image' FROM published_chapters
    UNION ALL SELECT card->>'image' FROM published_scenes
    UNION ALL SELECT pick->>'image' FROM visible_reader_scopes
        CROSS JOIN LATERAL jsonb_array_elements(picks) AS pick
) images WHERE reference IS NOT NULL AND reference <> '' ORDER BY reference;
