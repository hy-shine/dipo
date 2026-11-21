# Releasing Dipo

Dipo follows GitHub Flow with Semantic Versioning. Every change reaches `master` through a pull request that passes CI. A release is a version tag on a `master` commit; the tag starts `.github/workflows/release.yml`, which builds a signed disk image and an update feed for Apple silicon and for Intel and leaves them in a draft release for a person to check.

## Versions

The version is `version:` in `nsgo.yaml`. Nothing else holds it: `make -s version` prints it, and release.yml refuses a tag that does not match it.

Pick the next version by [Semantic Versioning](https://semver.org):

- A fix only: bump the patch (`0.1.0` to `0.1.1`).
- New behavior that stays compatible: bump the minor (`0.1.1` to `0.2.0`).
- A breaking change: bump the major from 1.0.0 on. Before 1.0.0, the SemVer spec lets any release break compatibility, so bump the minor for such a change to make it visible.

## Changes

- Work on a branch named for its topic, such as `release-assets`, and open a pull request into `master`.
- Commit messages and pull request titles follow Conventional Commits: `type(scope): subject`, subject in the imperative, title at most 50 characters. A rebase merge keeps each commit as written; a squash merge uses the pull request's title as the commit message.
- One concern per commit. A rebase merge keeps every commit in `master`'s history, so tidy a branch before merging it; a squash merge combines them into one, so split the pull request if its concerns should stay apart.
- `Lint` and `Test` must pass before a pull request can merge. CI runs on every push and every pull request; it never builds a release.
- Merge with the method the repository allows, which is set in the repository's settings.

## Releasing a version

1. Open a pull request that sets `version:` in `nsgo.yaml` to the new version, and merge it.
2. Write the release notes in Markdown to a file outside the repository, such as `notes.md`: what is new, what changed, and the requirements. The tag's message holds them, and the release shows them as written.
3. On an up-to-date `master`, check the version and tag it with the notes:

   ```sh
   git switch master && git pull --ff-only
   make -s version
   git tag -a vX.Y.Z --cleanup=verbatim -F notes.md
   git push origin vX.Y.Z
   ```

   `--cleanup=verbatim` keeps the notes' `##` headings, which `git tag` otherwise drops as comment lines. Check the message with `git tag --list vX.Y.Z --format='%(contents)'` before pushing.
4. Wait for the Release workflow in the Actions tab. It runs CI, builds the release with `make release`, and creates a draft release with six files: for each of Apple silicon (`macos-arm64`) and Intel (`macos-x64`), the disk image, its `.sha256` file and the appcast. It refuses a tag without a message, which would otherwise give the release the commit's message as its notes.
5. Check the draft before publishing it:
   - The notes are the tag's message.
   - `gh release view vX.Y.Z` lists exactly those six files.
   - Download each disk image and run `shasum -a 256 -c` against its `.sha256` file.
   - Open each disk image. macOS should not warn about it. The app still needs "Open Anyway" on its first launch, as the README says, because Dipo is not notarized.
   - On an Intel Mac, open the Intel disk image, pick a color and copy it. The release workflow builds the Intel app and checks its architecture, but runs neither app.
6. Publish the draft with `gh release edit vX.Y.Z --draft=false`. The update feed reads `releases/latest/download/`, so the new version reaches the app only after this step.
7. Test the update: on a Mac with the previous release installed, choose **Settings → About → Check Now**, and confirm the app updates and keeps its Screen Recording permission. This test cannot run before publishing, because the feed reads only published releases.

If step 7 fails, delete the release and the tag with `gh release delete vX.Y.Z --yes --cleanup-tag`. The feed falls back to the previous release. Fix the problem on a branch, then release a higher version. Do not reuse a tag.

## Before tagging

Run the checks locally on the commit to be tagged:

```sh
make check
make check-ui
```

`make check-ui` needs the macOS native test host and skips the menu bar test when the host has no menu bar.

## Secrets and signing

The Release workflow reads these from the repository settings (names only; the values are kept in a password manager):

- Secret `NSGO_UPDATES_KEY`: the private key that signs the update feed. Its public half is in `nsgo.yaml`.
- Secret `BUILD_CERTIFICATE_BASE64`: the code signing identity as a base64-encoded `.p12` file.
- Secret `P12_PASSWORD`: the password of that `.p12` file.
- Variable `CODESIGN_IDENTITY`: the name of the identity in the `.p12` file.

Keep a backup of the `.p12` file and the update key outside the repository. Losing the update key stops the feed from being signed; changing the signing identity would likely make existing installs ask for Screen Recording again (not tested).

## Known limits

- The disk image sets `LSMinimumSystemVersion` to 11.0 because nsgo writes that value and the build cannot override it in the image. The picker needs macOS 14, so the README says macOS 14 or later.
- The first release with an Intel build has no earlier Intel release to update from, so step 7 can test an Intel update only from the release after it.
- Dipo is not notarized.
