# typed: false
# frozen_string_literal: true

# Synth — intent-aware Git companion
# Homebrew formula for github.com/shyamsundaravssb/synth
#
# TO UPDATE THIS FORMULA FOR A NEW RELEASE:
#   1. Push a new version tag (e.g. v0.1.0) to GitHub
#   2. The release.yml workflow creates the release
#      and attaches archives with SHA256 hashes
#   3. Replace VERSION, url, and sha256 below with
#      values from the new release's checksums.txt

class Synth < Formula
  desc "Intent-aware Git companion — capture why changes happen"
  homepage "https://github.com/shyamsundaravssb/synth"
  version "REPLACE_WITH_VERSION"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/shyamsundaravssb/synth/"\
          "releases/download/vREPLACE_WITH_VERSION/"\
          "synth_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT"
    else
      url "https://github.com/shyamsundaravssb/synth/"\
          "releases/download/vREPLACE_WITH_VERSION/"\
          "synth_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/shyamsundaravssb/synth/"\
          "releases/download/vREPLACE_WITH_VERSION/"\
          "synth_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT"
    else
      url "https://github.com/shyamsundaravssb/synth/"\
          "releases/download/vREPLACE_WITH_VERSION/"\
          "synth_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT"
    end
  end

  def install
    bin.install "synth"
  end

  def caveats
    <<~EOS
      Synth requires a Git repository to work.
      Run in any Git repo:
        synth init

      For semantic search, download the embedding model
      (one-time, ~87MB):
        synth model download

      To start the background daemon:
        synth daemon start

      To install as a system service (auto-starts on login):
        synth daemon install-service

      Full documentation:
        https://github.com/shyamsundaravssb/synth
    EOS
  end

  test do
    system "#{bin}/synth", "--version"
  end
end
