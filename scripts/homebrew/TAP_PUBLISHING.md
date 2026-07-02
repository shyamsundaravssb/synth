# Publishing The Synth Homebrew Tap

## One-time setup (do this once, before first publish)

1. Create a new public GitHub repository named:
     homebrew-synth
   at: github.com/shyamsundaravssb/homebrew-synth

2. Clone it locally:
     git clone https://github.com/shyamsundaravssb/homebrew-synth

3. Copy scripts/homebrew/synth.rb into the new repo:
     cp <synth-repo>/scripts/homebrew/synth.rb \
        homebrew-synth/Formula/synth.rb

   (Create the Formula/ directory if needed)

## For each new release

1. Push a version tag to the synth repo:
     git tag v0.1.0
     git push origin v0.1.0

2. Wait for release.yml to complete and produce
   artifacts on the GitHub Releases page.

3. From the Releases page, copy the SHA256 hash
   for each platform archive from checksums.txt.

4. Update homebrew-synth/Formula/synth.rb:
   - Replace REPLACE_WITH_VERSION with the version
     number (without the leading v)
   - Replace each REPLACE_WITH_SHA256_FROM_CHECKSUMS_TXT
     with the correct SHA256 for that platform

5. Commit and push to homebrew-synth:
     cd homebrew-synth
     git add Formula/synth.rb
     git commit -m "synth v0.1.0"
     git push

## How users install once the tap is published

   brew tap shyamsundaravssb/synth
   brew install synth

## Testing the formula locally (Linux/macOS with Homebrew)

   brew install --formula ./Formula/synth.rb
