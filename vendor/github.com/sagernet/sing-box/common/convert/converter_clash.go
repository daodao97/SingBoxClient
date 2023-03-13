package convert

import (
	"github.com/sagernet/sing-box/common/xtype"
	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"gopkg.in/yaml.v2"
)

func ConvertsClash(buf []byte) (ob []option.Outbound, err error) {
	var proxies = struct {
		Proxies []xtype.Map
	}{}

	if err := yaml.Unmarshal(buf, &proxies); err != nil {
		return nil, err
	}

	for _, v := range proxies.Proxies {
		if v.GetString("server", "") == "127.0.0.1" {
			continue
		}
		switch v["type"] {
		case "vmess":
			ob = append(ob, option.Outbound{
				Type: constant.TypeVMess,
				Tag:  v.GetString("name", ""),
				VMessOptions: option.VMessOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     v.GetString("server", ""),
						ServerPort: v.GetUInt16("port", 0),
					},
					UUID:     v.GetString("uuid", ""),
					Security: v.GetString("cipher", ""),
					AlterId:  v.GetInt("alter_id", 0),
				},
			})
		case "ss":
			ob = append(ob, option.Outbound{
				Type: constant.TypeShadowsocks,
				Tag:  v.GetString("name", ""),
				ShadowsocksOptions: option.ShadowsocksOutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     v.GetString("server", ""),
						ServerPort: v.GetUInt16("port", 0),
					},
					Method:   v.GetString("cipher", ""),
					Password: v.GetString("password", ""),
				},
			})
		case "trojan":
		case "hysteria":
		case "shadowsocksr":
		case "vless":
		}
	}

	return ob, nil
}
