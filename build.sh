#!/usr/bin/env bash
set -x

export LDFLAGS="-L/usr/local/opt/openssl@3/lib"
export CPPFLAGS="-I/usr/local/opt/openssl@3/include"

gobuild="go build --tags with_quic,with_grpc,with_wireguard,with_shadowsocksr,with_ech,with_utls,with_acme,with_clash_api,with_gvisor"

function buildMac() {
  name=$1$([ -n "$2" ] && echo -$2 || echo )
  echo "start build mac-${name}"
  rm -rf icons.iconset
  mkdir icons.iconset
  echo "build macos icons"
  sips -z 16 16     icon/logo.png --out icons.iconset/icon_16x16.png      1>/dev/null
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_16x16@2x.png   1>/dev/null
  sips -z 32 32     icon/logo.png --out icons.iconset/icon_32x32.png      1>/dev/null
  sips -z 64 64     icon/logo.png --out icons.iconset/icon_32x32@2x.png   1>/dev/null
  sips -z 128 128   icon/logo.png --out icons.iconset/icon_128x128.png    1>/dev/null
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_128x128@2x.png 1>/dev/null
  sips -z 256 256   icon/logo.png --out icons.iconset/icon_256x256.png    1>/dev/null
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_256x256@2x.png 1>/dev/null
  sips -z 512 512   icon/logo.png --out icons.iconset/icon_512x512.png    1>/dev/null
  sips -z 1024 1024 icon/logo.png --out icons.iconset/icon_512x512@2x.png 1>/dev/null

  rm -rf build/SingBox.app
  cp -rf build/meta/SingBox.app build

  iconutil -c icns icons.iconset -o build/SingBox.app/Contents/Resources/icon.icns
  rm -rf icons.iconset

  $(env GOOS=darwin GOARCH=$1 $([ -n "$2" ] && echo GOAMD64=$2 || echo ) CGO_ENABLED=1 $gobuild -o build/SingBox.app/Contents/MacOS/sbox .)

  (cd build && zip -r SingBox-mac-${name}.zip SingBox.app 1>/dev/null)

  rm -rf build/SingBox.app
  echo "success !"
}

function buildWin() {
    echo "start build win-amd64"
    rsrc -manifest build/meta/win/sbox.exe.manifest -ico icon/logo.ico -o sbox.exe.syso
    # brew info mingw-w64
    $(env GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="x86_64-w64-mingw32-gcc" $gobuild -ldflags -H=windowsgui -o build/SingBox.exe ./)

    (cd build && zip -r SingBox-win-amd64.zip SingBox.exe 1>/dev/null)

    rm sbox.exe.syso
    rm build/SingBox.exe
  echo "success !"
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
    buildMac amd64
    buildMac amd64 v3
  ;;
  m1)
    buildMac arm64
  ;;
  win)
    buildWin arm64
  ;;
  *)
    buildMac amd64
    buildMac amd64 v3
    buildMac arm64
    buildWin
  ;;
esac

open build

#if [ -z "${v}" ] || [ -z "${p}" ]; then
#    usage
#fi



