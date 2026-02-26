#!/bin/bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
FRAMEWORKS_DIR="$ROOT/appletv/Frameworks"
export PATH="$HOME/go/bin:$PATH"

echo "=== Snake.io Build Script ==="
echo ""

# --- Helper ---
cmd_exists() { command -v "$1" &>/dev/null; }

# --- Build standalone server ---
build_server() {
    echo "[1/4] Building standalone server..."
    cd "$ROOT"
    go build -o "$ROOT/server/snake-server" ./server/
    echo "  -> server/snake-server"
}

# --- Build gomobile framework for tvOS ---
build_mobile() {
    echo "[2/4] Building gomobile framework for tvOS..."

    # gomobile bind requires Xcode.app (not just Command Line Tools)
    if [ ! -d "/Applications/Xcode.app" ]; then
        echo "  ERROR: Xcode.app is required for gomobile bind."
        echo "  Install Xcode from the App Store, then run:"
        echo "    sudo xcode-select -s /Applications/Xcode.app/Contents/Developer"
        exit 1
    fi

    if ! cmd_exists gomobile; then
        echo "  gomobile not found. Installing..."
        go install golang.org/x/mobile/cmd/gomobile@latest
        go install golang.org/x/mobile/cmd/gobind@latest
        gomobile init
    fi

    mkdir -p "$FRAMEWORKS_DIR"

    # Step 1: Build iOS xcframework with gomobile
    echo "  Building iOS xcframework with gomobile..."
    cd "$ROOT"
    gomobile bind \
        -target=ios \
        -o "$FRAMEWORKS_DIR/Mobile.xcframework" \
        ./mobile/

    # Step 2: Patch platform tags from iOS -> tvOS
    # gomobile doesn't support tvOS natively, but the arm64 binary is
    # identical between iOS and tvOS — only the Mach-O platform tag differs.
    echo "  Patching platform tags for tvOS..."
    rm -rf "$FRAMEWORKS_DIR/Mobile-tvOS.xcframework"

    IOS_XCF="$FRAMEWORKS_DIR/Mobile.xcframework"
    TVOS_XCF="$FRAMEWORKS_DIR/Mobile-tvOS.xcframework"

    # Device (arm64)
    TVOS_DEVICE_FW="$TVOS_XCF/tvos-arm64/Mobile.framework"
    mkdir -p "$TVOS_DEVICE_FW/Headers" "$TVOS_DEVICE_FW/Modules"
    TMP=$(mktemp -d)
    lipo -thin arm64 "$IOS_XCF/ios-arm64/Mobile.framework/Mobile" -output "$TMP/ios.a"
    python3 "$ROOT/patch-platform.py" "$TMP/ios.a" "$TMP/tvos.a" tvos
    lipo -create "$TMP/tvos.a" -output "$TVOS_DEVICE_FW/Mobile"
    rm -rf "$TMP"
    cp "$IOS_XCF/ios-arm64/Mobile.framework/Headers/"* "$TVOS_DEVICE_FW/Headers/"
    cp "$IOS_XCF/ios-arm64/Mobile.framework/Modules/module.modulemap" "$TVOS_DEVICE_FW/Modules/"
    cat > "$TVOS_DEVICE_FW/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key><string>Mobile</string>
    <key>CFBundleIdentifier</key><string>snake.io.Mobile</string>
    <key>CFBundleName</key><string>Mobile</string>
    <key>CFBundleVersion</key><string>1.0.0</string>
    <key>CFBundlePackageType</key><string>FMWK</string>
    <key>MinimumOSVersion</key><string>16.0</string>
</dict>
</plist>
PLIST

    # Simulator (arm64 + x86_64)
    TVOS_SIM_FW="$TVOS_XCF/tvos-arm64_x86_64-simulator/Mobile.framework"
    mkdir -p "$TVOS_SIM_FW/Headers" "$TVOS_SIM_FW/Modules"
    SIM_SRC="$IOS_XCF/ios-arm64_x86_64-simulator/Mobile.framework/Mobile"
    TMP=$(mktemp -d)
    for arch in arm64 x86_64; do
        lipo -thin "$arch" "$SIM_SRC" -output "$TMP/${arch}_ios.a" 2>/dev/null || continue
        python3 "$ROOT/patch-platform.py" "$TMP/${arch}_ios.a" "$TMP/${arch}_tvos.a" tvos-sim
    done
    if [ -f "$TMP/arm64_tvos.a" ] && [ -f "$TMP/x86_64_tvos.a" ]; then
        lipo -create "$TMP/arm64_tvos.a" "$TMP/x86_64_tvos.a" -output "$TVOS_SIM_FW/Mobile"
    elif [ -f "$TMP/arm64_tvos.a" ]; then
        lipo -create "$TMP/arm64_tvos.a" -output "$TVOS_SIM_FW/Mobile"
    fi
    rm -rf "$TMP"
    cp "$IOS_XCF/ios-arm64_x86_64-simulator/Mobile.framework/Headers/"* "$TVOS_SIM_FW/Headers/"
    cp "$IOS_XCF/ios-arm64_x86_64-simulator/Mobile.framework/Modules/module.modulemap" "$TVOS_SIM_FW/Modules/"
    cat > "$TVOS_SIM_FW/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key><string>Mobile</string>
    <key>CFBundleIdentifier</key><string>snake.io.Mobile</string>
    <key>CFBundleName</key><string>Mobile</string>
    <key>CFBundleVersion</key><string>1.0.0</string>
    <key>CFBundlePackageType</key><string>FMWK</string>
    <key>MinimumOSVersion</key><string>16.0</string>
</dict>
</plist>
PLIST

    # XCFramework manifest
    cat > "$TVOS_XCF/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>AvailableLibraries</key>
    <array>
        <dict>
            <key>BinaryPath</key><string>Mobile.framework/Mobile</string>
            <key>LibraryIdentifier</key><string>tvos-arm64</string>
            <key>LibraryPath</key><string>Mobile.framework</string>
            <key>SupportedArchitectures</key><array><string>arm64</string></array>
            <key>SupportedPlatform</key><string>tvos</string>
        </dict>
        <dict>
            <key>BinaryPath</key><string>Mobile.framework/Mobile</string>
            <key>LibraryIdentifier</key><string>tvos-arm64_x86_64-simulator</string>
            <key>LibraryPath</key><string>Mobile.framework</string>
            <key>SupportedArchitectures</key><array><string>arm64</string><string>x86_64</string></array>
            <key>SupportedPlatform</key><string>tvos</string>
            <key>SupportedPlatformVariant</key><string>simulator</string>
        </dict>
    </array>
    <key>CFBundlePackageType</key><string>XFWK</string>
    <key>XCFrameworkFormatVersion</key><string>1.0</string>
</dict>
</plist>
PLIST

    echo "  -> appletv/Frameworks/Mobile-tvOS.xcframework"
}

# --- Generate Xcode project ---
build_xcode() {
    echo "[3/4] Generating Xcode project..."

    if ! cmd_exists xcodegen; then
        echo "  xcodegen not found. Install with: brew install xcodegen"
        exit 1
    fi

    cd "$ROOT/appletv/SnakeTV"
    xcodegen generate
    echo "  -> appletv/SnakeTV/SnakeTV.xcodeproj"
    cd "$ROOT"
}

# --- Build tvOS app ---
build_app() {
    echo "[4/4] Building tvOS app..."
    xcodebuild \
        -project "$ROOT/appletv/SnakeTV/SnakeTV.xcodeproj" \
        -scheme SnakeTV \
        -destination 'platform=tvOS Simulator,name=Apple TV 4K (3rd generation)' \
        build 2>&1 | tail -3
}

# --- Main ---
case "${1:-all}" in
    server)
        build_server
        ;;
    mobile)
        build_mobile
        ;;
    xcode)
        build_xcode
        ;;
    app)
        build_app
        ;;
    all)
        build_server
        build_mobile
        build_xcode
        build_app
        echo ""
        echo "=== Done! ==="
        echo "Open appletv/SnakeTV/SnakeTV.xcodeproj in Xcode"
        echo "Select your Apple TV as the target device and hit Run"
        ;;
    *)
        echo "Usage: $0 [server|mobile|xcode|app|all]"
        echo ""
        echo "  server  - Build standalone Go server binary"
        echo "  mobile  - Build gomobile .xcframework for tvOS"
        echo "  xcode   - Generate Xcode project with xcodegen"
        echo "  app     - Build tvOS app for simulator"
        echo "  all     - Build everything (default)"
        exit 1
        ;;
esac
