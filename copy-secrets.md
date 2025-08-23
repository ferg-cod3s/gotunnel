# Copy Signing Secrets from Rune to Gotunnel

## Step 1: Get Secrets from Rune Repository
Go to: https://github.com/ferg-cod3s/rune/settings/secrets/actions

Copy the values for these secrets:
- `MACOS_CERTIFICATE`
- `MACOS_CERTIFICATE_PWD` 
- `MACOS_SIGN_IDENTITY`
- `MACOS_NOTARY_ISSUER_ID`
- `MACOS_NOTARY_KEY_ID`
- `MACOS_NOTARY_KEY`

## Step 2: Add Secrets to Gotunnel Repository
Go to: https://github.com/ferg-cod3s/gotunnel/settings/secrets/actions

Click "New repository secret" for each and paste the values:

### Required for Basic Signing:
- **Name:** `MACOS_CERTIFICATE`
  **Value:** [Same as rune - base64 encoded .p12 file]

- **Name:** `MACOS_CERTIFICATE_PWD` 
  **Value:** [Same as rune - .p12 password]

- **Name:** `MACOS_SIGN_IDENTITY`
  **Value:** [Same as rune - Developer ID like "Developer ID Application: Your Name (TEAMID)"]

### Optional for Notarization (Recommended):
- **Name:** `MACOS_NOTARY_ISSUER_ID`
  **Value:** [Same as rune - App Store Connect issuer ID]

- **Name:** `MACOS_NOTARY_KEY_ID`
  **Value:** [Same as rune - App Store Connect key ID]

- **Name:** `MACOS_NOTARY_KEY`
  **Value:** [Same as rune - App Store Connect private key]

## Step 3: Test Signing
1. Create a new release tag: `git tag v0.1.1-beta && git push origin v0.1.1-beta`
2. Create a GitHub release from that tag
3. Check the build logs for: "✅ Signing macOS binary with identity"
4. Download the macOS binary and verify it's signed: `codesign -v gotunnel-*-darwin-*`

## What This Enables:
✅ macOS binaries will be signed with your Developer ID
✅ No "unidentified developer" warnings for users  
✅ Homebrew installation will be smoother
✅ Enterprise environments will accept the binaries
✅ Optional: Notarization for extra trust

The gotunnel CI pipeline is already configured - it just needs these secrets!