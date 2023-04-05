#!/bin/bash

# First, ensure that the github token is set
if [[ -z $GITHUB_TOKEN ]]; then
  echo Github token not found. 
  echo "Please set your github token via \"export GITHUB_TOKEN=<your-gh-token>\" and try again."
  exit 1
fi

current_tag=`git describe --tags --abbrev=0`
echo The current tag is $current_tag.
echo What would you like the new tag to be?
read new_tag
tag_message=`git --no-pager log -1 --pretty=%s`

echo Pushing $new_tag: $tag_message. 
read -p "Continue? [y/N] " yn
case $yn in
    [Yy]* ) echo Pushing to remote...;;
    * ) echo Exiting...; exit;;
esac

# Push tag
git tag -a $new_tag -m $tag_message
git push origin $new_tag

read -p "Build and release cli binaries? [y/N] " yn
case $yn in
    [Yy]* ) echo Releasing...;;
    * ) echo Exiting...; exit;;
esac

# Build & release, setting the tag as the lekko cli version via ldflags.
export CLI_TAG=$new_tag
goreleaser release --rm-dist
