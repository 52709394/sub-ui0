package xray

import (
	"fmt"

	"strings"
	"sub-ui/change"
	"sub-ui/proxy"
	"sub-ui/proxy/protocol"
	"sub-ui/random"
	"sub-ui/read"
	"sub-ui/setup"
	"sub-ui/users"
)

func (rF RealityFallbacks) setData(config *users.Config) {
	for i := range rF.Fallbacks {
		for j := range config.Inbounds {
			switch v := rF.Fallbacks[i].Dest.(type) {
			case string:
				if config.Inbounds[j].ServiceListen != string(v) {
					continue
				}
			case uint16:
				if config.Inbounds[j].ServicePort != uint16(v) {
					continue
				}
			default:
				continue
			}

			x := rF.Fallbacks[i].Index
			if config.Inbounds[j].Transport != nil {
				if config.Inbounds[x].Transport == nil {
					config.Inbounds[x].Transport = new(users.Transport)
				}
				config.Inbounds[x].Network = config.Inbounds[j].Network
				config.Inbounds[x].Transport = config.Inbounds[j].Transport
			}
			config.Inbounds[x].Users = config.Inbounds[j].Users

		}
	}

}

func (inbound Inbound) getData(usersInbound *users.Inbound) string {
	protocol := inbound.Protocol
	usersInbound.Protocol = protocol

	usersInbound.ServiceListen = inbound.Listen
	usersInbound.ServicePort = inbound.Port

	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" {
		usersInbound.Port = fmt.Sprintf("%d", inbound.Port)
	}

	switch protocol {
	case "vmess", "vless", "trojan":

		usersInbound.Network = inbound.StreamSettings.Network
		usersInbound.FixedSecurity = false

		if usersInbound.Network != "raw" && usersInbound.Network != "tcp" {
			usersInbound.Transport = new(users.Transport)
		}

		switch usersInbound.Network {
		case "raw":
			usersInbound.Network = "tcp"
		case "grpc":
			usersInbound.Transport.ServiceName = inbound.StreamSettings.GrpcSettings.ServiceName
		case "h2", "http":
			usersInbound.Network = "http"
			usersInbound.Transport.Path = inbound.StreamSettings.HttpSettings.Path
			if len(inbound.StreamSettings.HttpSettings.Host) > 0 {
				usersInbound.Transport.Host = ""
				for _, host := range inbound.StreamSettings.HttpSettings.Host {
					usersInbound.Transport.Host += host + ","
				}
				usersInbound.Transport.Host = strings.TrimRight(usersInbound.Transport.Host, ",")
			}
		case "ws":
			usersInbound.Transport.Path = inbound.StreamSettings.WsSettings.Path
		case "splithttp":
			usersInbound.Transport.Path = inbound.StreamSettings.SplithttpSettings.Path
		case "xhttp":
			usersInbound.Transport.Path = inbound.StreamSettings.XhttpSettings.Path
		case "httpupgrade":
			usersInbound.Transport.Path = inbound.StreamSettings.HttpupgradeSettings.Path
		}

		if protocol == "vless" {
			if inbound.StreamSettings.Security == "reality" {
				usersInbound.Security = "reality"
				usersInbound.FixedSecurity = true
				usersInbound.Reality = new(users.Reality)
				usersInbound.Reality.Sni = inbound.StreamSettings.RealitySettings.ServerNames[0]
				if publicKey, err := change.GetPublicKey(inbound.StreamSettings.RealitySettings.PrivateKey); err == nil {
					usersInbound.Reality.PublicKey = publicKey
				}
				if len(inbound.StreamSettings.RealitySettings.ShortIds) > 0 {
					usersInbound.Reality.ShortId = inbound.StreamSettings.RealitySettings.ShortIds[0]
				}
			}
		}

		if inbound.StreamSettings.Security == "tls" {
			usersInbound.Security = "tls"
			usersInbound.FixedSecurity = true
			if len(inbound.StreamSettings.TlsSettings.Alpn) > 0 {
				usersInbound.Tls = new(users.Tls)
				usersInbound.Tls.Alpn = ""
				for _, alpn := range inbound.StreamSettings.TlsSettings.Alpn {
					usersInbound.Tls.Alpn += alpn + ","
				}
				usersInbound.Tls.Alpn = strings.TrimRight(usersInbound.Tls.Alpn, ",")
			}
		}

		usersInbound.Fingerprint = setup.ConfigData.Users.UtlsFp

		return protocol
	}

	if protocol == "shadowsocks" {
		if inbound.Settings.Method != "" {
			path, err := random.GenerateStrings(16)
			if err != nil {
				fmt.Println("随机url路径错误:", err)
				return ""
			}
			usersInbound.Users = append(usersInbound.Users, users.User{
				Name:     proxy.OnlyName + "-" + fmt.Sprintf("%d", inbound.Port),
				Password: inbound.Settings.Password,
				Method:   inbound.Settings.Method,
				Static:   false,
				UserPath: path,
			})
		}
		usersInbound.Fingerprint = setup.ConfigData.Users.UtlsFp
		return protocol
	}

	return ""
}

func (config Config) RenewData(mod string) error {

	usersConfig := users.Config{}

	read.GetJsonData(setup.ConfigData.Proxy.Config, &config)

	var newUsersInbound users.Inbound
	var path string
	var base64 string
	var total int
	var name string
	var err error

	rF := RealityFallbacks{}

	path = ""

	for i := range config.Inbounds {

		if config.Inbounds[i].Tag == "" {
			continue
		}

		protocol := config.Inbounds[i].getData(&newUsersInbound)

		if protocol == "" {
			newUsersInbound = users.Inbound{}
			continue
		}

		base64 = change.ToBase64(config.Inbounds[i].Tag)

		base64 = strings.ReplaceAll(base64, "+", "252B")
		base64 = strings.ReplaceAll(base64, "/", "252F")
		base64 = strings.ReplaceAll(base64, "=", "253D")

		newUsersInbound.Tag = config.Inbounds[i].Tag
		newUsersInbound.TagPath = base64

		total = len(config.Inbounds[i].Settings.Clients)

		for j := range config.Inbounds[i].Settings.Clients {

			if config.Inbounds[i].Settings.Clients[j].Email == "" && total != 1 {
				continue
			}

			if config.Inbounds[i].Settings.Clients[j].Email == "" {
				//name = proxy.OnlyName + "-" + fmt.Sprintf("%d", i)
				name = proxy.OnlyName + "-" + fmt.Sprintf("%d", config.Inbounds[i].Port)
			} else {
				name = config.Inbounds[i].Settings.Clients[j].Email
			}

			path, err = random.GenerateStrings(16)
			if err != nil {
				fmt.Println("随机url路径错误:", err)
				return err
			}

			newUsersInbound.Users = append(newUsersInbound.Users, users.User{
				Name:     name,
				Static:   false,
				UserPath: path,
			})

			n := len(newUsersInbound.Users) - 1
			switch protocol {
			case "vmess":
				newUsersInbound.Users[n].UUID = config.Inbounds[i].Settings.Clients[j].Id
			case "vless":
				newUsersInbound.Users[n].UUID = config.Inbounds[i].Settings.Clients[j].Id
				newUsersInbound.Users[n].Flow = config.Inbounds[i].Settings.Clients[j].Flow
			case "trojan":
				newUsersInbound.Users[n].Password = config.Inbounds[i].Settings.Clients[j].Password
			case "shadowsocks":
				newUsersInbound.Users[n].Method = config.Inbounds[i].Settings.Clients[j].Method
				newUsersInbound.Users[n].Password = config.Inbounds[i].Settings.Clients[j].Password
			}
		}

		if len(newUsersInbound.Users) == 0 {
			if newUsersInbound.Security == "reality" &&
				len(config.Inbounds[i].Settings.Fallbacks) == 1 {
				rF.Fallbacks = append(rF.Fallbacks, RealityFallback{
					Index: len(usersConfig.Inbounds),
					Dest:  config.Inbounds[i].Settings.Fallbacks[0].Dest,
				})

			} else {
				newUsersInbound.Hide = true
			}

		} else {
			newUsersInbound.Hide = false
		}

		usersConfig.Inbounds = append(usersConfig.Inbounds, newUsersInbound)
		newUsersInbound = users.Inbound{}
	}

	rF.setData(&usersConfig)

	if setup.ConfigData.Static.Enabled {
		usersConfig.SetStaticUrl()
	}

	path = ""
	if mod == "renew" {
		usersConfig.SetOldData()
	}

	err = usersConfig.SavedConfig()
	if err != nil {
		return err
	}

	return nil
}

func (config LConfig) GetCurrentData(p *protocol.Config, tag string, userName string) {
	read.GetJsonData(setup.ConfigData.Proxy.Config, &config)
	isUpdata := false
OuterLoop:
	for i := range config.Inbounds {
		if config.Inbounds[i].Tag != tag {
			continue
		}

		if len(config.Inbounds[i].Settings.Clients) == 1 {
			if config.Inbounds[i].Settings.Clients[0].Email == "" {
				switch config.Inbounds[i].Protocol {
				case "vmess", "vless":
					if p.UserUUID != nil {
						if *p.UserUUID != config.Inbounds[i].Settings.Clients[0].Id {
							*p.UserUUID = config.Inbounds[i].Settings.Clients[0].Id
							isUpdata = true
						}
					}

				case "trojan", "shadowsocks":
					if p.UserPassword != nil {
						if *p.UserPassword != config.Inbounds[i].Settings.Clients[0].Password {
							*p.UserPassword = config.Inbounds[i].Settings.Clients[0].Password
							isUpdata = true
						}
					}
				}

				break OuterLoop
			}
		} else if config.Inbounds[i].Protocol == "shadowsocks" {
			if len(config.Inbounds[i].Settings.Clients) == 0 && config.Inbounds[i].Settings.Password != "" {
				if p.UserPassword != nil {
					if *p.UserPassword != config.Inbounds[i].Settings.Password {
						*p.UserPassword = config.Inbounds[i].Settings.Password
						isUpdata = true
					}
				}
				break OuterLoop
			}
		}

		for j := range config.Inbounds[i].Settings.Clients {
			if config.Inbounds[i].Settings.Clients[j].Email == userName {
				switch config.Inbounds[i].Protocol {
				case "vmess", "vless":
					if p.UserUUID != nil {
						if *p.UserUUID != config.Inbounds[i].Settings.Clients[j].Id {
							*p.UserUUID = config.Inbounds[i].Settings.Clients[j].Id
							isUpdata = true
						}
					}
				case "trojan", "shadowsocks":
					if p.UserPassword != nil {
						if *p.UserPassword != config.Inbounds[i].Settings.Clients[j].Password {
							*p.UserPassword = config.Inbounds[i].Settings.Clients[j].Password
							isUpdata = true
						}
					}
				}
				break OuterLoop
			}

		}
	}

	if isUpdata {
		users.ConfigData.SavedConfig()
	}
}
