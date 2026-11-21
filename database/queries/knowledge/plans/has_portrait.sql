SELECT EXISTS(SELECT FROM edition_reader_art WHERE world_slug=$1 AND kind='character' AND item_slug=$2)
