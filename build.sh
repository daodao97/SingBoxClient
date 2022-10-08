#!/usr/bin/env bash
set -x

function buildMac() {
  rm -rf icons.iconset
  mkdir icons.iconset
  sips -z 16 16     icon/logo.png --out icons.iconset/icon_16x16.png
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_16x16@2x.png
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_32x32.png
  sips -z 64 64     icon/logo.png --out icons.iconset/icon_32x32@2x.png
  sips -z 128 128   icon/logo.png --out icons.iconset/icon_128x128.png
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_128x128@2x.png
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_256x256.png
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_256x256@2x.png
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_512x512.png
  sips -z 1024 1024 icon/logo.png --out icons.iconset/icon_512x512@2x.png

  rm -rf build/SingBox.app
  cp -rf build/meta/SingBox.app build

  iconutil -c icns icons.iconset -o build/SingBox.app/Contents/Resources/icon.icns
  rm -rf icons.iconset

  env GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags with_clash_api -o build/SingBox.app/Contents/MacOS/sbox .
  (cd build && zip -r SingBox-mac-amd64.zip SingBox.app)
  rm -rf build/SingBox.app
  echo "macos app build success"
}

function buildMacM1() {
  rm -rf icons.iconset
  mkdir icons.iconset
  sips -z 16 16     icon/logo.png --out icons.iconset/icon_16x16.png
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_16x16@2x.png
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_32x32.png
  sips -z 64 64     icon/logo.png --out icons.iconset/icon_32x32@2x.png
  sips -z 128 128   icon/logo.png --out icons.iconset/icon_128x128.png
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_128x128@2x.png
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_256x256.png
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_256x256@2x.png
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_512x512.png
  sips -z 1024 1024 icon/logo.png --out icons.iconset/icon_512x512@2x.png

  rm -rf build/SingBox.app
  cp -rf build/meta/SingBox.app build

  iconutil -c icns icons.iconset -o build/SingBox.app/Contents/Resources/icon.icns
  rm -rf icons.iconset

  env GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags with_clash_api -o build/SingBox.app/Contents/MacOS/sbox .
  (cd build && zip -r SingBox-mac-arm64.zip SingBox.app)
  rm -rf build/SingBox.app
  echo "macos app build success"
}

function buildWin() {
    rsrc -manifest build/meta/win/sbox.exe.manifest -ico icon/logo.ico -o sbox.exe.syso
    # brew info mingw-w64
    env GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="x86_64-w64-mingw32-gcc" go build -tags with_clash_api -ldflags -H=windowsgui -o build/SingBox.exe ./
    (cd build && zip -r SingBox-win-amd64.zip SingBox.exe)
    rm sbox.exe.syso
    rm build/SingBox.exe
  echo "win app build success"
}

usage() { echo "Usage: $0 [-v string] [-p <string>]" 1>&2; exit 1; }

while getopts ":v:p:h:" o; do
    case "${o}" in
        v)
            v=${OPTARG}
            ;;
        p)
            p=${OPTARG}
            ;;
        h)
            usage
            ;;
        *)
            ;;
    esac
done
shift $((OPTIND-1))

case "${p}" in
  mac)
    buildMac
  ;;
  m1)
    buildMacM1
  ;;
  win)
    buildWin
  ;;
  *)
    buildMac
    buildMacM1
    buildWin
  ;;
esac

open build

#if [ -z "${v}" ] || [ -z "${p}" ]; then
#    usage
#fi



