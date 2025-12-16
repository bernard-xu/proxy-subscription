package models

import (
	"strings"
	"time"
)

// Subscription 订阅模型
type Subscription struct {
	BaseModel
	Name            string    `json:"name" gorm:"not null"`
	URL             string    `json:"url" gorm:"not null"`
	Type            string    `json:"type" gorm:"not null"` // 支持的订阅类型，如v2ray, trojan, ss等
	Enabled         bool      `json:"enabled" gorm:"default:true"`
	LastUpdated     time.Time `json:"lastUpdated"`
	Proxies         []Proxy   `json:"proxies,omitempty" gorm:"foreignKey:SubscriptionID"`
	ValidProxyCount int       `json:"valid_proxy_count"`
}

// Proxy 代理节点模型
type Proxy struct {
	BaseModel
	SubscriptionID uint   `json:"subscription_id" gorm:"not null"`
	Name           string `json:"name" gorm:"not null"`
	Type           string `json:"type" gorm:"not null"` // v2ray, ss, trojan等
	Server         string `json:"server" gorm:"not null"`
	Port           int    `json:"port" gorm:"not null"`
	UUID           string `json:"uuid"`
	Password       string `json:"password"`
	Method         string `json:"method"`
	Network        string `json:"network"`
	Path           string `json:"path"`
	Host           string `json:"host"`
	TLS            bool   `json:"tls"`
	SNI            string `json:"sni"`
	ALPN           string `json:"alpn"`
	Plugin         string `json:"plugin"`                     // Shadowsocks插件名称
	PluginOpts     string `json:"plugin_opts"`                // Shadowsocks插件选项
	AllowInsecure  bool   `json:"allow_insecure"`             // 是否允许不安全连接（跳过证书验证）
	RawConfig      string `json:"rawConfig" gorm:"type:text"` // 存储原始配置
	DisplayName    string `json:"display_name" gorm:"-"`      // 格式化后的显示名称，不存储到数据库
}

// GetDisplayName 根据名称获取格式化的显示名称
// 参考 abs.cpp 的 DisplayCountry 逻辑，将代理名称转换为标准化的国家名称显示格式
func (p *Proxy) GetDisplayName() string {
	if p.Name == "" {
		return "未知"
	}

	// 国家代码到中文名称的映射
	codeToChineseName := map[string]string{
		"CN": "中国", "JP": "日本", "KR": "韩国", "KP": "朝鲜", "MN": "蒙古",
		"VN": "越南", "LA": "老挝", "KH": "柬埔寨", "MM": "缅甸", "TH": "泰国",
		"MY": "马来西亚", "SG": "新加坡", "ID": "印度尼西亚", "PH": "菲律宾",
		"BN": "文莱", "TL": "东帝汶", "NP": "尼泊尔", "BT": "不丹", "BD": "孟加拉",
		"IN": "印度", "PK": "巴基斯坦", "LK": "斯里兰卡", "MV": "马尔代夫",
		"KZ": "哈萨克斯坦", "KG": "吉尔吉斯斯坦", "TJ": "塔吉克斯坦",
		"UZ": "乌兹别克斯坦", "TM": "土库曼斯坦", "AF": "阿富汗",
		"IQ": "伊拉克", "IR": "伊朗", "SY": "叙利亚", "JO": "约旦",
		"LB": "黎巴嫩", "IL": "以色列", "PS": "巴勒斯坦", "SA": "沙特阿拉伯",
		"BH": "巴林", "QA": "卡塔尔", "KW": "科威特", "AE": "阿联酋",
		"OM": "阿曼", "YE": "也门", "GE": "格鲁吉亚", "AM": "亚美尼亚",
		"AZ": "阿塞拜疆", "TR": "土耳其", "CY": "塞浦路斯",
		"FI": "芬兰", "SE": "瑞典", "NO": "挪威", "IS": "冰岛", "DK": "丹麦",
		"EE": "爱沙尼亚", "LV": "拉脱维亚", "LT": "立陶宛", "BY": "白俄罗斯",
		"RU": "俄罗斯", "UA": "乌克兰", "PL": "波兰", "CZ": "捷克",
		"SK": "斯洛伐克", "HU": "匈牙利", "DE": "德国", "AT": "奥地利",
		"CH": "瑞士", "LI": "列支敦士登", "GB": "英国", "IE": "爱尔兰",
		"NL": "荷兰", "BE": "比利时", "LU": "卢森堡", "FR": "法国",
		"MC": "摩纳哥", "IT": "意大利", "VA": "梵蒂冈", "SM": "圣马力诺",
		"MT": "马耳他", "ES": "西班牙", "PT": "葡萄牙", "AD": "安道尔",
		"GR": "希腊", "BG": "保加利亚", "RO": "罗马尼亚", "RS": "塞尔维亚",
		"HR": "克罗地亚", "SI": "斯洛文尼亚", "BA": "波黑", "ME": "黑山",
		"AL": "阿尔巴尼亚", "MK": "北马其顿",
		"EG": "埃及", "LY": "利比亚", "TN": "突尼斯", "DZ": "阿尔及利亚",
		"MA": "摩洛哥", "SD": "苏丹", "SS": "南苏丹", "ET": "埃塞俄比亚",
		"ER": "厄立特里亚", "SO": "索马里", "DJ": "吉布提", "KE": "肯尼亚",
		"TZ": "坦桑尼亚", "UG": "乌干达", "RW": "卢旺达", "BI": "布隆迪",
		"SC": "塞舌尔", "TD": "乍得", "CF": "中非", "CM": "喀麦隆",
		"GQ": "赤道几内亚", "GA": "加蓬", "CG": "刚果共和国",
		"CD": "刚果民主共和国", "ST": "圣多美和普林西比",
		"MR": "毛里塔尼亚", "SN": "塞内加尔", "GM": "冈比亚", "ML": "马里",
		"BF": "布基纳法索", "GN": "几内亚", "GW": "几内亚比绍",
		"CV": "佛得角", "SL": "塞拉利昂", "LR": "利比里亚", "CI": "科特迪瓦",
		"GH": "加纳", "TG": "多哥", "BJ": "贝宁", "NE": "尼日尔",
		"NG": "尼日利亚", "ZM": "赞比亚", "AO": "安哥拉", "ZW": "津巴布韦",
		"MW": "马拉维", "MZ": "莫桑比克", "BW": "博茨瓦纳", "NA": "纳米比亚",
		"ZA": "南非", "SZ": "斯威士兰", "LS": "莱索托", "MG": "马达加斯加",
		"KM": "科摩罗", "MU": "毛里求斯",
		"CA": "加拿大", "US": "美国", "GU": "关岛", "MX": "墨西哥",
		"GT": "危地马拉", "BZ": "伯利兹", "SV": "萨尔瓦多", "HN": "洪都拉斯",
		"NI": "尼加拉瓜", "CR": "哥斯达黎加", "PA": "巴拿马", "CU": "古巴",
		"JM": "牙买加", "HT": "海地", "DO": "多米尼加", "BS": "巴哈马",
		"BB": "巴巴多斯", "KN": "圣基茨和尼维斯", "LC": "圣卢西亚",
		"VC": "圣文森特和格林纳丁斯", "GD": "格林纳达",
		"TT": "特立尼达和多巴哥", "CO": "哥伦比亚", "VE": "委内瑞拉",
		"GY": "圭亚那", "SR": "苏里南", "EC": "厄瓜多尔", "PE": "秘鲁",
		"BO": "玻利维亚", "BR": "巴西", "CL": "智利", "AR": "阿根廷",
		"UY": "乌拉圭", "PY": "巴拉圭",
		"AU": "澳大利亚", "NZ": "新西兰", "PG": "巴布亚新几内亚",
		"SB": "所罗门群岛", "VU": "瓦努阿图", "FJ": "斐济", "KI": "基里巴斯",
		"NR": "瑙鲁", "FM": "密克罗尼西亚", "MH": "马绍尔群岛",
		"PW": "帕劳", "WS": "萨摩亚", "TO": "汤加", "TV": "图瓦卢",
		"TW": "台湾", "HK": "香港", "MO": "澳门", "XK": "科索沃",
		"EH": "西撒哈拉", "PR": "波多黎各", "AQ": "南极", "GL": "格陵兰",
		"RE": "留尼汪", "GF": "法属圭亚那", "PF": "法属波利尼西亚",
		"MF": "法属圣马丁", "PM": "圣皮埃尔和密克隆群岛",
	}

	// 国家名称到代码的映射（中文、英文、emoji、别名）
	countryMap := map[string]string{
		// 中文名称
		"日本": "JP", "韩国": "KR", "朝鲜": "KP", "蒙古": "MN", "越南": "VN",
		"老挝": "LA", "柬埔寨": "KH", "缅甸": "MM", "泰国": "TH",
		"马来西亚": "MY", "新加坡": "SG", "印度尼西亚": "ID", "菲律宾": "PH",
		"文莱": "BN", "东帝汶": "TL", "尼泊尔": "NP", "不丹": "BT",
		"孟加拉": "BD", "印度": "IN", "巴基斯坦": "PK", "斯里兰卡": "LK",
		"马尔代夫": "MV", "哈萨克斯坦": "KZ", "吉尔吉斯斯坦": "KG",
		"塔吉克斯坦": "TJ", "乌兹别克斯坦": "UZ", "土库曼斯坦": "TM",
		"阿富汗": "AF", "伊拉克": "IQ", "伊朗": "IR", "叙利亚": "SY",
		"约旦": "JO", "黎巴嫩": "LB", "以色列": "IL", "巴勒斯坦": "PS",
		"沙特阿拉伯": "SA", "巴林": "BH", "卡塔尔": "QA", "科威特": "KW",
		"阿联酋": "AE", "阿曼": "OM", "也门": "YE", "格鲁吉亚": "GE",
		"亚美尼亚": "AM", "阿塞拜疆": "AZ", "土耳其": "TR", "塞浦路斯": "CY",
		"芬兰": "FI", "瑞典": "SE", "挪威": "NO", "冰岛": "IS", "丹麦": "DK",
		"爱沙尼亚": "EE", "拉脱维亚": "LV", "立陶宛": "LT", "白俄罗斯": "BY",
		"俄罗斯": "RU", "乌克兰": "UA", "波兰": "PL", "捷克": "CZ",
		"斯洛伐克": "SK", "匈牙利": "HU", "德国": "DE", "奥地利": "AT",
		"瑞士": "CH", "列支敦士登": "LI", "英国": "GB", "爱尔兰": "IE",
		"荷兰": "NL", "比利时": "BE", "卢森堡": "LU", "法国": "FR",
		"摩纳哥": "MC", "意大利": "IT", "梵蒂冈": "VA", "圣马力诺": "SM",
		"马耳他": "MT", "西班牙": "ES", "葡萄牙": "PT", "安道尔": "AD",
		"希腊": "GR", "保加利亚": "BG", "罗马尼亚": "RO", "塞尔维亚": "RS",
		"克罗地亚": "HR", "斯洛文尼亚": "SI", "波黑": "BA", "黑山": "ME",
		"阿尔巴尼亚": "AL", "北马其顿": "MK",
		"埃及": "EG", "利比亚": "LY", "突尼斯": "TN", "阿尔及利亚": "DZ",
		"摩洛哥": "MA", "苏丹": "SD", "南苏丹": "SS", "埃塞俄比亚": "ET",
		"厄立特里亚": "ER", "索马里": "SO", "吉布提": "DJ", "肯尼亚": "KE",
		"坦桑尼亚": "TZ", "乌干达": "UG", "卢旺达": "RW", "布隆迪": "BI",
		"塞舌尔": "SC", "乍得": "TD", "中非": "CF", "喀麦隆": "CM",
		"赤道几内亚": "GQ", "加蓬": "GA", "刚果共和国": "CG",
		"刚果民主共和国": "CD", "圣多美和普林西比": "ST",
		"毛里塔尼亚": "MR", "塞内加尔": "SN", "冈比亚": "GM", "马里": "ML",
		"布基纳法索": "BF", "几内亚": "GN", "几内亚比绍": "GW",
		"佛得角": "CV", "塞拉利昂": "SL", "利比里亚": "LR", "科特迪瓦": "CI",
		"加纳": "GH", "多哥": "TG", "贝宁": "BJ", "尼日尔": "NE",
		"尼日利亚": "NG", "赞比亚": "ZM", "安哥拉": "AO", "津巴布韦": "ZW",
		"马拉维": "MW", "莫桑比克": "MZ", "博茨瓦纳": "BW", "纳米比亚": "NA",
		"南非": "ZA", "斯威士兰": "SZ", "莱索托": "LS", "马达加斯加": "MG",
		"科摩罗": "KM", "毛里求斯": "MU",
		"加拿大": "CA", "美国": "US", "关岛": "GU", "墨西哥": "MX",
		"危地马拉": "GT", "伯利兹": "BZ", "萨尔瓦多": "SV", "洪都拉斯": "HN",
		"尼加拉瓜": "NI", "哥斯达黎加": "CR", "巴拿马": "PA", "古巴": "CU",
		"牙买加": "JM", "海地": "HT", "多米尼加": "DO", "巴哈马": "BS",
		"巴巴多斯": "BB", "圣基茨和尼维斯": "KN", "圣卢西亚": "LC",
		"圣文森特和格林纳丁斯": "VC", "格林纳达": "GD",
		"特立尼达和多巴哥": "TT", "哥伦比亚": "CO", "委内瑞拉": "VE",
		"圭亚那": "GY", "苏里南": "SR", "厄瓜多尔": "EC", "秘鲁": "PE",
		"玻利维亚": "BO", "巴西": "BR", "智利": "CL", "阿根廷": "AR",
		"乌拉圭": "UY", "巴拉圭": "PY",
		"澳大利亚": "AU", "新西兰": "NZ", "巴布亚新几内亚": "PG",
		"所罗门群岛": "SB", "瓦努阿图": "VU", "斐济": "FJ", "基里巴斯": "KI",
		"瑙鲁": "NR", "密克罗尼西亚": "FM", "马绍尔群岛": "MH",
		"帕劳": "PW", "萨摩亚": "WS", "汤加": "TO", "图瓦卢": "TV",
		"台湾": "TW", "香港": "HK", "澳门": "MO", "科索沃": "XK",
		"西撒哈拉": "EH", "波多黎各": "PR", "南极": "AQ", "格陵兰": "GL",
		"留尼汪": "RE", "法属圭亚那": "GF", "法属波利尼西亚": "PF",
		"法属圣马丁": "MF", "圣皮埃尔和密克隆群岛": "PM",
		// 国旗emoji
		"🇭🇰": "HK", "🇺🇸": "US", "🇯🇵": "JP", "🇷🇺": "RU", "🇨🇭": "CH",
		"🇫🇷": "FR", "🇬🇧": "GB", "🇸🇬": "SG", "🇰🇷": "KR", "🇩🇪": "DE",
		"🇮🇹": "IT", "🇪🇸": "ES", "🇨🇦": "CA", "🇦🇺": "AU", "🇧🇷": "BR",
		"🇮🇳": "IN", "🇹🇼": "TW", "🇲🇴": "MO", "🇹🇭": "TH", "🇲🇾": "MY",
		"🇮🇩": "ID", "🇵🇭": "PH", "🇻🇳": "VN", "🇳🇱": "NL", "🇸🇪": "SE",
		"🇳🇴": "NO", "🇫🇮": "FI", "🇩🇰": "DK", "🇦🇹": "AT", "🇵🇱": "PL",
		"🇹🇷": "TR", "🇺🇦": "UA", "🇮🇪": "IE", "🇧🇪": "BE", "🇵🇹": "PT",
		"🇬🇷": "GR", "🇿🇦": "ZA", "🇪🇬": "EG", "🇮🇱": "IL", "🇸🇦": "SA",
		"🇦🇪": "AE", "🇲🇽": "MX", "🇦🇷": "AR", "🇨🇱": "CL", "🇳🇿": "NZ",
		"🇮🇸": "IS", "🇱🇺": "LU", "🇨🇿": "CZ", "🇭🇺": "HU", "🇷🇴": "RO",
		"🇧🇬": "BG", "🇭🇷": "HR", "🇸🇮": "SI", "🇸🇰": "SK", "🇱🇹": "LT",
		"🇱🇻": "LV", "🇪🇪": "EE",
		// 英文名称
		"Hong Kong": "HK", "HongKong": "HK", "United States": "US",
		"USA": "US", "America": "US", "Japan": "JP", "Russia": "RU",
		"Russian": "RU", "Switzerland": "CH", "France": "FR", "French": "FR",
		"United Kingdom": "GB", "UK": "GB", "Britain": "GB", "England": "GB",
		"Singapore": "SG", "Korea": "KR", "South Korea": "KR",
		"Germany": "DE", "German": "DE", "Italy": "IT", "Italian": "IT",
		"Spain": "ES", "Spanish": "ES", "Canada": "CA", "Canadian": "CA",
		"Australia": "AU", "Australian": "AU", "Brazil": "BR", "Brazilian": "BR",
		"India": "IN", "Indian": "IN", "Taiwan": "TW", "Macau": "MO",
		"Macao": "MO", "Thailand": "TH", "Thai": "TH", "Malaysia": "MY",
		"Indonesian": "ID", "Philippines": "PH", "Vietnam": "VN",
		"Vietnamese": "VN", "Netherlands": "NL", "Holland": "NL", "Dutch": "NL",
		"Sweden": "SE", "Swedish": "SE", "Norway": "NO", "Norwegian": "NO",
		"Finland": "FI", "Finnish": "FI", "Denmark": "DK", "Danish": "DK",
		"Austria": "AT", "Austrian": "AT", "Poland": "PL", "Polish": "PL",
		"Turkey": "TR", "Turkish": "TR", "Ukraine": "UA", "Ukrainian": "UA",
		"Ireland": "IE", "Irish": "IE", "Belgium": "BE", "Belgian": "BE",
		"Portugal": "PT", "Portuguese": "PT", "Greece": "GR", "Greek": "GR",
		"South Africa": "ZA", "Egypt": "EG", "Egyptian": "EG",
		"Israel": "IL", "Israeli": "IL", "Saudi Arabia": "SA",
		"UAE": "AE", "Mexico": "MX", "Mexican": "MX", "Argentina": "AR",
		"Chilean": "CL", "New Zealand": "NZ", "Iceland": "IS",
		// 地区别名（只保留不会与中文名称重复的）
		"港": "HK", "沙特": "SA",
	}

	// 特殊处理印度（避免与印度尼西亚冲突）
	if strings.Contains(p.Name, "印度") && !strings.Contains(p.Name, "印度尼西亚") {
		return "印度 (IN)"
	}

	nameLower := strings.ToLower(p.Name)
	nameUpper := strings.ToUpper(p.Name)

	// 遍历国家映射，寻找匹配项
	for key, code := range countryMap {
		matched := false

		// 直接匹配（包含emoji和中文）
		if strings.Contains(p.Name, key) {
			matched = true
		} else if len(key) > 0 && (key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z') {
			// 大小写不敏感的英文匹配
			keyLower := strings.ToLower(key)
			keyUpper := strings.ToUpper(key)
			if strings.Contains(nameLower, keyLower) || strings.Contains(nameUpper, keyUpper) {
				matched = true
			}
		}

		if matched {
			// 返回统一格式：中文名称 + 代码
			chineseName := codeToChineseName[code]
			if chineseName == "" {
				chineseName = "未知"
			}
			return chineseName + " (" + code + ")"
		}
	}

	return "未知"
}

// AfterFind GORM hook，在查询后自动设置 DisplayName
func (p *Proxy) AfterFind() error {
	p.DisplayName = p.GetDisplayName()
	return nil
}
