INSERT INTO home_world_slots SELECT coalesce(max(position),0)+1,$1 FROM home_world_slots
