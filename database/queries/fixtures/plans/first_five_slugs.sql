SELECT string_agg(id||':'||(spec->>'slug'),',' ORDER BY number) FROM episode_plans WHERE number<=5
