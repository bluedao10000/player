SELECT EXISTS(SELECT FROM episode_plan_manifests WHERE world_slug=$1 AND checksum=$2)
