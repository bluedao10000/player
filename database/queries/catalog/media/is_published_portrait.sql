SELECT EXISTS(SELECT FROM edition_reader_art WHERE kind='character' AND image=$1)
