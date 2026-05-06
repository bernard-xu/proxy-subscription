package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
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
	IsCustom       bool   `json:"is_custom" gorm:"default:false;index"`
	ManualOverride bool   `json:"manual_override" gorm:"default:false;index"`
	SourceKey      string `json:"source_key" gorm:"index"`
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

// BuildSourceKey returns a stable identity for a proxy parsed from a subscription.
func (p *Proxy) BuildSourceKey() string {
	rawConfig := strings.TrimSpace(p.RawConfig)
	if rawConfig != "" {
		return "raw:" + hashSourceKey(rawConfig)
	}

	parts := []string{
		strings.ToLower(strings.TrimSpace(p.Type)),
		strings.ToLower(strings.TrimSpace(p.Name)),
		strings.ToLower(strings.TrimSpace(p.Server)),
		strconv.Itoa(p.Port),
		strings.TrimSpace(p.UUID),
		strings.TrimSpace(p.Password),
		strings.ToLower(strings.TrimSpace(p.Method)),
	}
	return "fields:" + hashSourceKey(strings.Join(parts, "\x00"))
}

func hashSourceKey(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
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
		"NC": "新喀里多尼亚", "WF": "瓦利斯和富图纳", "YT": "马约特",
		"TF": "法属南部和南极领地",
		"VG": "英属维尔京群岛", "KY": "开曼群岛", "MS": "蒙特塞拉特",
		"AI": "安圭拉", "TC": "特克斯和凯科斯群岛", "BM": "百慕大",
		"GI": "直布罗陀", "FK": "福克兰群岛", "SH": "圣赫勒拿",
		"PN": "皮特凯恩群岛", "IO": "英属印度洋领地",
		"AW": "阿鲁巴", "CW": "库拉索", "SX": "荷属圣马丁", "BQ": "博奈尔",
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
		"新喀里多尼亚": "NC", "瓦利斯和富图纳": "WF", "马约特": "YT",
		"法属南部和南极领地": "TF",
		"英属维尔京群岛":   "VG", "开曼群岛": "KY", "蒙特塞拉特": "MS",
		"安圭拉": "AI", "特克斯和凯科斯群岛": "TC", "百慕大": "BM",
		"直布罗陀": "GI", "福克兰群岛": "FK", "圣赫勒拿": "SH",
		"皮特凯恩群岛": "PN", "英属印度洋领地": "IO",
		"阿鲁巴": "AW", "库拉索": "CW", "荷属圣马丁": "SX", "博奈尔": "BQ",
		// 中文别名（避免单字冲突，使用更明确的别名，不与中文名称重复）
		"港": "HK", "台": "TW", "澳": "MO", "日": "JP", "韩": "KR",
		"朝": "KP", "蒙": "MN", "越": "VN", "老": "LA", "柬": "KH", "缅": "MM",
		"泰": "TH", "马来": "MY", "新": "SG", "印尼": "ID", "菲": "PH", "文": "BN",
		"东帝": "TL", "尼泊": "NP", "不": "BT", "孟": "BD", "印": "IN", "巴基": "PK",
		"斯里": "LK", "马代": "MV", "哈萨": "KZ", "吉尔": "KG", "塔吉": "TJ",
		"乌兹": "UZ", "土库": "TM", "阿富": "AF", "伊拉": "IQ",
		"叙": "SY", "约": "JO", "黎": "LB", "以": "IL", "巴勒": "PS",
		"沙特": "SA", "卡": "QA", "科": "KW", "阿联": "AE",
		"也": "YE", "格鲁": "GE", "亚美": "AM", "阿塞": "AZ",
		"土": "TR", "塞浦": "CY", "芬": "FI", "挪": "NO", "冰": "IS",
		"丹": "DK", "爱沙": "EE", "拉": "LV", "立": "LT", "白俄": "BY",
		"俄": "RU", "乌": "UA", "波": "PL", "捷": "CZ", "斯洛": "SK",
		"匈": "HU", "德": "DE", "奥": "AT", "列": "LI", "英": "GB",
		"荷": "NL", "比": "BE", "卢": "LU", "法": "FR", "摩纳": "MC",
		"意": "IT", "梵": "VA", "圣马": "SM", "马耳": "MT", "西": "ES", "葡": "PT",
		"安道": "AD", "希": "GR", "保": "BG", "罗": "RO", "塞尔": "RS", "克": "HR",
		"斯洛文": "SI", "黑": "ME", "阿尔": "AL", "北马": "MK",
		"埃": "EG", "利": "LY", "突": "TN", "阿尔及": "DZ", "摩洛": "MA", "苏": "SD",
		"南苏": "SS", "埃塞": "ET", "厄立": "ER", "索": "SO", "吉": "DJ", "肯": "KE",
		"坦": "TZ", "乌干": "UG", "卢旺": "RW", "布": "BI", "塞舌": "SC", "乍": "TD",
		"喀": "CM", "赤": "GQ", "刚": "CG", "刚民": "CD",
		"圣多": "ST", "毛里": "MR", "塞内": "SN", "冈": "GM", "马": "ML", "布基": "BF",
		"几": "GN", "几比": "GW", "佛": "CV", "塞拉": "SL", "利比": "LR", "科特": "CI",
		"多": "TG", "贝": "BJ", "尼日": "NE", "尼日利": "NG", "赞": "ZM",
		"安哥": "AO", "津": "ZW", "马拉": "MW", "莫": "MZ", "博": "BW", "纳": "NA",
		"斯威": "SZ", "莱": "LS", "马达": "MG", "科摩": "KM", "毛里求": "MU",
		"加": "CA", "美": "US", "关": "GU", "墨": "MX", "危": "GT", "伯": "BZ",
		"萨": "SV", "洪": "HN", "尼加": "NI", "哥斯": "CR", "巴拿": "PA", "古": "CU",
		"牙": "JM", "海": "HT", "多米": "DO", "巴哈": "BS", "巴巴": "BB", "圣基": "KN",
		"圣卢": "LC", "圣文": "VC", "格林": "GD", "特立": "TT", "哥伦": "CO", "委": "VE",
		"圭": "GY", "苏里": "SR", "厄瓜": "EC", "秘": "PE", "玻": "BO", "巴": "BR",
		"智": "CL", "阿根": "AR", "乌拉": "UY", "巴拉": "PY", "澳洲": "AU", "纽": "NZ",
		"巴新": "PG", "所": "SB", "瓦": "VU", "斐": "FJ", "基": "KI", "瑙": "NR",
		"密": "FM", "马绍": "MH", "帕": "PW", "萨摩": "WS", "汤": "TO", "图": "TV",
		"新喀": "NC", "瓦富": "WF", "法南": "TF",
		"英维": "VG", "开曼": "KY", "蒙特": "MS", "安圭": "AI", "特凯": "TC",
		"百": "BM", "直": "GI", "福": "FK", "圣赫": "SH", "皮": "PN", "英印": "IO",
		"阿鲁": "AW", "库拉": "CW", "荷属": "SX", "博奈": "BQ",
		// 国旗emoji（补充所有国家）
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
		"🇱🇻": "LV", "🇪🇪": "EE", "🇨🇳": "CN", "🇰🇵": "KP", "🇲🇳": "MN",
		"🇱🇦": "LA", "🇰🇭": "KH", "🇲🇲": "MM", "🇧🇳": "BN", "🇹🇱": "TL",
		"🇳🇵": "NP", "🇧🇹": "BT", "🇧🇩": "BD", "🇵🇰": "PK", "🇱🇰": "LK",
		"🇲🇻": "MV", "🇰🇿": "KZ", "🇰🇬": "KG", "🇹🇯": "TJ", "🇺🇿": "UZ",
		"🇹🇲": "TM", "🇦🇫": "AF", "🇮🇶": "IQ", "🇮🇷": "IR", "🇸🇾": "SY",
		"🇯🇴": "JO", "🇱🇧": "LB", "🇵🇸": "PS", "🇧🇭": "BH", "🇶🇦": "QA",
		"🇰🇼": "KW", "🇴🇲": "OM", "🇾🇪": "YE", "🇬🇪": "GE", "🇦🇲": "AM",
		"🇦🇿": "AZ", "🇨🇾": "CY", "🇧🇾": "BY", "🇱🇮": "LI", "🇲🇨": "MC",
		"🇻🇦": "VA", "🇸🇲": "SM", "🇲🇹": "MT", "🇦🇩": "AD", "🇷🇸": "RS",
		"🇧🇦": "BA", "🇲🇪": "ME", "🇦🇱": "AL", "🇲🇰": "MK", "🇱🇾": "LY",
		"🇹🇳": "TN", "🇩🇿": "DZ", "🇲🇦": "MA", "🇸🇩": "SD", "🇸🇸": "SS",
		"🇪🇹": "ET", "🇪🇷": "ER", "🇸🇴": "SO", "🇩🇯": "DJ", "🇰🇪": "KE",
		"🇹🇿": "TZ", "🇺🇬": "UG", "🇷🇼": "RW", "🇧🇮": "BI", "🇸🇨": "SC",
		"🇹🇩": "TD", "🇨🇫": "CF", "🇨🇲": "CM", "🇬🇶": "GQ", "🇬🇦": "GA",
		"🇨🇬": "CG", "🇨🇩": "CD", "🇸🇹": "ST", "🇲🇷": "MR", "🇸🇳": "SN",
		"🇬🇲": "GM", "🇲🇱": "ML", "🇧🇫": "BF", "🇬🇳": "GN", "🇬🇼": "GW",
		"🇨🇻": "CV", "🇸🇱": "SL", "🇱🇷": "LR", "🇨🇮": "CI", "🇬🇭": "GH",
		"🇹🇬": "TG", "🇧🇯": "BJ", "🇳🇪": "NE", "🇳🇬": "NG", "🇿🇲": "ZM",
		"🇦🇴": "AO", "🇿🇼": "ZW", "🇲🇼": "MW", "🇲🇿": "MZ", "🇧🇼": "BW",
		"🇳🇦": "NA", "🇸🇿": "SZ", "🇱🇸": "LS", "🇲🇬": "MG", "🇰🇲": "KM",
		"🇲🇺": "MU", "🇬🇺": "GU", "🇬🇹": "GT", "🇧🇿": "BZ", "🇸🇻": "SV",
		"🇭🇳": "HN", "🇳🇮": "NI", "🇨🇷": "CR", "🇵🇦": "PA", "🇨🇺": "CU",
		"🇯🇲": "JM", "🇭🇹": "HT", "🇩🇴": "DO", "🇧🇸": "BS", "🇧🇧": "BB",
		"🇰🇳": "KN", "🇱🇨": "LC", "🇻🇨": "VC", "🇬🇩": "GD", "🇹🇹": "TT",
		"🇨🇴": "CO", "🇻🇪": "VE", "🇬🇾": "GY", "🇸🇷": "SR", "🇪🇨": "EC",
		"🇵🇪": "PE", "🇧🇴": "BO", "🇺🇾": "UY", "🇵🇾": "PY", "🇵🇬": "PG",
		"🇸🇧": "SB", "🇻🇺": "VU", "🇫🇯": "FJ", "🇰🇮": "KI", "🇳🇷": "NR",
		"🇫🇲": "FM", "🇲🇭": "MH", "🇵🇼": "PW", "🇼🇸": "WS", "🇹🇴": "TO",
		"🇹🇻": "TV", "🇽🇰": "XK", "🇪🇭": "EH", "🇵🇷": "PR", "🇦🇶": "AQ",
		"🇬🇱": "GL", "🇷🇪": "RE", "🇬🇫": "GF", "🇵🇫": "PF", "🇲🇫": "MF",
		"🇵🇲": "PM", "🇳🇨": "NC", "🇼🇫": "WF", "🇾🇹": "YT", "🇹🇫": "TF",
		"🇻🇬": "VG", "🇰🇾": "KY", "🇲🇸": "MS", "🇦🇮": "AI", "🇹🇨": "TC",
		"🇧🇲": "BM", "🇬🇮": "GI", "🇫🇰": "FK", "🇸🇭": "SH", "🇵🇳": "PN",
		"🇮🇴": "IO", "🇦🇼": "AW", "🇨🇼": "CW", "🇸🇽": "SX", "🇧🇶": "BQ",
		// 英文名称（补充更多变体和别名）
		"Hong Kong": "HK", "HongKong": "HK", "HK": "HK", "HKG": "HK",
		"United States": "US", "USA": "US", "America": "US", "US": "US",
		"Japan": "JP", "JPN": "JP", "JAP": "JP",
		"Russia": "RU", "Russian": "RU", "RUS": "RU", "Russian Federation": "RU",
		"Switzerland": "CH", "CHE": "CH", "Swiss": "CH",
		"France": "FR", "French": "FR", "FRA": "FR",
		"United Kingdom": "GB", "UK": "GB", "Britain": "GB", "England": "GB",
		"Great Britain": "GB", "GBR": "GB", "British": "GB",
		"Singapore": "SG", "SGP": "SG", "Sing": "SG",
		"Korea": "KR", "South Korea": "KR", "KOR": "KR", "ROK": "KR",
		"Germany": "DE", "German": "DE", "DEU": "DE", "Deutschland": "DE",
		"Italy": "IT", "Italian": "IT", "ITA": "IT",
		"Spain": "ES", "Spanish": "ES", "ESP": "ES",
		"Canada": "CA", "Canadian": "CA", "CAN": "CA",
		"Australia": "AU", "Australian": "AU", "AUS": "AU",
		"Brazil": "BR", "Brazilian": "BR", "BRA": "BR",
		"India": "IN", "Indian": "IN", "IND": "IN",
		"Taiwan": "TW", "TWN": "TW", "ROC": "TW", "Formosa": "TW",
		"Macau": "MO", "Macao": "MO", "MAC": "MO",
		"Thailand": "TH", "Thai": "TH", "THA": "TH",
		"Malaysia": "MY", "MYS": "MY", "Malay": "MY",
		"Indonesia": "ID", "Indonesian": "ID", "IDN": "ID",
		"Philippines": "PH", "PHL": "PH", "Filipino": "PH",
		"Vietnam": "VN", "Vietnamese": "VN", "VNM": "VN",
		"Netherlands": "NL", "Holland": "NL", "Dutch": "NL", "NLD": "NL",
		"Sweden": "SE", "Swedish": "SE", "SWE": "SE",
		"Norway": "NO", "Norwegian": "NO", "NOR": "NO",
		"Finland": "FI", "Finnish": "FI", "FIN": "FI",
		"Denmark": "DK", "Danish": "DK", "DNK": "DK",
		"Austria": "AT", "Austrian": "AT", "AUT": "AT",
		"Poland": "PL", "Polish": "PL", "POL": "PL",
		"Turkey": "TR", "Turkish": "TR", "TUR": "TR",
		"Ukraine": "UA", "Ukrainian": "UA", "UKR": "UA",
		"Ireland": "IE", "Irish": "IE", "IRL": "IE",
		"Belgium": "BE", "Belgian": "BE", "BEL": "BE",
		"Portugal": "PT", "Portuguese": "PT", "PRT": "PT",
		"Greece": "GR", "Greek": "GR", "GRC": "GR",
		"South Africa": "ZA", "ZAF": "ZA", "RSA": "ZA",
		"Egypt": "EG", "Egyptian": "EG", "EGY": "EG",
		"Israel": "IL", "Israeli": "IL", "ISR": "IL",
		"Saudi Arabia": "SA", "SAU": "SA", "KSA": "SA",
		"UAE": "AE", "United Arab Emirates": "AE", "ARE": "AE",
		"Mexico": "MX", "Mexican": "MX", "MEX": "MX",
		"Argentina": "AR", "ARG": "AR", "Argentine": "AR",
		"Chile": "CL", "Chilean": "CL", "CHL": "CL",
		"New Zealand": "NZ", "NZL": "NZ", "Kiwi": "NZ",
		"Iceland": "IS", "ISL": "IS", "Icelandic": "IS",
		"China": "CN", "Chinese": "CN", "CHN": "CN", "PRC": "CN",
		"North Korea": "KP", "DPRK": "KP", "PRK": "KP",
		"Mongolia": "MN", "MNG": "MN", "Mongolian": "MN",
		"Laos": "LA", "LAO": "LA", "Lao": "LA",
		"Cambodia": "KH", "KHM": "KH", "Kampuchea": "KH",
		"Myanmar": "MM", "Burma": "MM", "MMR": "MM",
		"Brunei": "BN", "BRN": "BN", "Brunei Darussalam": "BN",
		"East Timor": "TL", "Timor-Leste": "TL", "TLS": "TL",
		"Nepal": "NP", "NPL": "NP", "Nepalese": "NP",
		"Bhutan": "BT", "BTN": "BT", "Bhutanese": "BT",
		"Bangladesh": "BD", "BGD": "BD", "Bangladeshi": "BD",
		"Pakistan": "PK", "PAK": "PK", "Pakistani": "PK",
		"Sri Lanka": "LK", "LKA": "LK", "Ceylon": "LK",
		"Maldives": "MV", "MDV": "MV", "Maldivian": "MV",
		"Kazakhstan": "KZ", "KAZ": "KZ", "Kazakh": "KZ",
		"Kyrgyzstan": "KG", "KGZ": "KG", "Kyrgyz": "KG",
		"Tajikistan": "TJ", "TJK": "TJ", "Tajik": "TJ",
		"Uzbekistan": "UZ", "UZB": "UZ", "Uzbek": "UZ",
		"Turkmenistan": "TM", "TKM": "TM", "Turkmen": "TM",
		"Afghanistan": "AF", "AFG": "AF", "Afghan": "AF",
		"Iraq": "IQ", "IRQ": "IQ", "Iraqi": "IQ",
		"Iran": "IR", "IRN": "IR", "Persia": "IR", "Persian": "IR",
		"Syria": "SY", "SYR": "SY", "Syrian": "SY",
		"Jordan": "JO", "JOR": "JO", "Jordanian": "JO",
		"Lebanon": "LB", "LBN": "LB", "Lebanese": "LB",
		"Palestine": "PS", "PSE": "PS", "Palestinian": "PS",
		"Bahrain": "BH", "BHR": "BH", "Bahraini": "BH",
		"Qatar": "QA", "QAT": "QA", "Qatari": "QA",
		"Kuwait": "KW", "KWT": "KW", "Kuwaiti": "KW",
		"Oman": "OM", "OMN": "OM", "Omani": "OM",
		"Yemen": "YE", "YEM": "YE", "Yemeni": "YE",
		"Georgia": "GE", "GEO": "GE", "Georgian": "GE",
		"Armenia": "AM", "ARM": "AM", "Armenian": "AM",
		"Azerbaijan": "AZ", "AZE": "AZ", "Azerbaijani": "AZ",
		"Cyprus": "CY", "CYP": "CY", "Cypriot": "CY",
		"Estonia": "EE", "EST": "EE", "Estonian": "EE",
		"Latvia": "LV", "LVA": "LV", "Latvian": "LV",
		"Lithuania": "LT", "LTU": "LT", "Lithuanian": "LT",
		"Belarus": "BY", "BLR": "BY", "Belarusian": "BY", "White Russia": "BY",
		"Czech Republic": "CZ", "Czech": "CZ", "CZE": "CZ", "Czechia": "CZ",
		"Slovakia": "SK", "Slovak": "SK", "SVK": "SK",
		"Hungary": "HU", "Hungarian": "HU", "HUN": "HU",
		"Luxembourg": "LU", "LUX": "LU", "Luxembourgish": "LU",
		"Monaco": "MC", "MCO": "MC", "Monacan": "MC",
		"Vatican": "VA", "Vatican City": "VA", "VAT": "VA",
		"San Marino": "SM", "SMR": "SM", "Sammarinese": "SM",
		"Malta": "MT", "MLT": "MT", "Maltese": "MT",
		"Andorra": "AD", "AND": "AD", "Andorran": "AD",
		"Bulgaria": "BG", "BGR": "BG", "Bulgarian": "BG",
		"Romania": "RO", "ROU": "RO", "Romanian": "RO",
		"Serbia": "RS", "SRB": "RS", "Serbian": "RS",
		"Croatia": "HR", "HRV": "HR", "Croatian": "HR",
		"Slovenia": "SI", "SVN": "SI", "Slovenian": "SI",
		"Bosnia": "BA", "Bosnia and Herzegovina": "BA", "BIH": "BA",
		"Montenegro": "ME", "MNE": "ME", "Montenegrin": "ME",
		"Albania": "AL", "ALB": "AL", "Albanian": "AL",
		"North Macedonia": "MK", "Macedonia": "MK", "MKD": "MK",
		"Libya": "LY", "LBY": "LY", "Libyan": "LY",
		"Tunisia": "TN", "TUN": "TN", "Tunisian": "TN",
		"Algeria": "DZ", "DZA": "DZ", "Algerian": "DZ",
		"Morocco": "MA", "MAR": "MA", "Moroccan": "MA",
		"Sudan": "SD", "SDN": "SD", "Sudanese": "SD",
		"South Sudan": "SS", "SSD": "SS", "South Sudanese": "SS",
		"Ethiopia": "ET", "ETH": "ET", "Ethiopian": "ET",
		"Eritrea": "ER", "ERI": "ER", "Eritrean": "ER",
		"Somalia": "SO", "SOM": "SO", "Somali": "SO",
		"Djibouti": "DJ", "DJI": "DJ", "Djiboutian": "DJ",
		"Kenya": "KE", "KEN": "KE", "Kenyan": "KE",
		"Tanzania": "TZ", "TZA": "TZ", "Tanzanian": "TZ",
		"Uganda": "UG", "UGA": "UG", "Ugandan": "UG",
		"Rwanda": "RW", "RWA": "RW", "Rwandan": "RW",
		"Burundi": "BI", "BDI": "BI", "Burundian": "BI",
		"Seychelles": "SC", "SYC": "SC", "Seychellois": "SC",
		"Chad": "TD", "TCD": "TD", "Chadian": "TD",
		"Central African Republic": "CF", "CAR": "CF", "CAF": "CF",
		"Cameroon": "CM", "CMR": "CM", "Cameroonian": "CM",
		"Equatorial Guinea": "GQ", "GNQ": "GQ", "Equatoguinean": "GQ",
		"Gabon": "GA", "GAB": "GA", "Gabonese": "GA",
		"Congo": "CG", "Republic of the Congo": "CG", "COG": "CG",
		"DRC": "CD", "Democratic Republic of the Congo": "CD", "COD": "CD",
		"Sao Tome and Principe": "ST", "STP": "ST",
		"Mauritania": "MR", "MRT": "MR", "Mauritanian": "MR",
		"Senegal": "SN", "SEN": "SN", "Senegalese": "SN",
		"Gambia": "GM", "GMB": "GM", "Gambian": "GM",
		"Mali": "ML", "MLI": "ML", "Malian": "ML",
		"Burkina Faso": "BF", "BFA": "BF", "Burkinabe": "BF",
		"Guinea": "GN", "GIN": "GN", "Guinean": "GN",
		"Guinea-Bissau": "GW", "GNB": "GW",
		"Cape Verde": "CV", "CPV": "CV", "Cabo Verde": "CV",
		"Sierra Leone": "SL", "SLE": "SL",
		"Liberia": "LR", "LBR": "LR", "Liberian": "LR",
		"Ivory Coast": "CI", "Cote d'Ivoire": "CI", "CIV": "CI",
		"Ghana": "GH", "GHA": "GH", "Ghanian": "GH",
		"Togo": "TG", "TGO": "TG", "Togolese": "TG",
		"Benin": "BJ", "BEN": "BJ", "Beninese": "BJ",
		"Niger": "NE", "NER": "NE", "Nigerien": "NE",
		"Nigeria": "NG", "NGA": "NG", "Nigerian": "NG",
		"Zambia": "ZM", "ZMB": "ZM", "Zambian": "ZM",
		"Angola": "AO", "AGO": "AO", "Angolan": "AO",
		"Zimbabwe": "ZW", "ZWE": "ZW", "Zimbabwean": "ZW",
		"Malawi": "MW", "MWI": "MW", "Malawian": "MW",
		"Mozambique": "MZ", "MOZ": "MZ", "Mozambican": "MZ",
		"Botswana": "BW", "BWA": "BW", "Botswanan": "BW",
		"Namibia": "NA", "NAM": "NA", "Namibian": "NA",
		"Eswatini": "SZ", "Swaziland": "SZ", "SWZ": "SZ",
		"Lesotho": "LS", "LSO": "LS", "Basotho": "LS",
		"Madagascar": "MG", "MDG": "MG", "Malagasy": "MG",
		"Comoros": "KM", "COM": "KM", "Comorian": "KM",
		"Mauritius": "MU", "MUS": "MU", "Mauritian": "MU",
		"Guam": "GU", "GUM": "GU", "Guamanian": "GU",
		"Guatemala": "GT", "GTM": "GT", "Guatemalan": "GT",
		"Belize": "BZ", "BLZ": "BZ", "Belizean": "BZ",
		"El Salvador": "SV", "SLV": "SV", "Salvadoran": "SV",
		"Honduras": "HN", "HND": "HN", "Honduran": "HN",
		"Nicaragua": "NI", "NIC": "NI", "Nicaraguan": "NI",
		"Costa Rica": "CR", "CRI": "CR", "Costa Rican": "CR",
		"Panama": "PA", "PAN": "PA", "Panamanian": "PA",
		"Cuba": "CU", "CUB": "CU", "Cuban": "CU",
		"Jamaica": "JM", "JAM": "JM", "Jamaican": "JM",
		"Haiti": "HT", "HTI": "HT", "Haitian": "HT",
		"Dominican Republic": "DO", "DOM": "DO", "Dominican": "DO",
		"Bahamas": "BS", "BSH": "BS", "Bahamian": "BS",
		"Barbados": "BB", "BRB": "BB", "Barbadian": "BB",
		"Saint Kitts and Nevis": "KN", "KNA": "KN", "St Kitts": "KN",
		"Saint Lucia": "LC", "LCA": "LC", "St Lucia": "LC",
		"Saint Vincent": "VC", "VCT": "VC", "St Vincent": "VC",
		"Grenada": "GD", "GRD": "GD", "Grenadian": "GD",
		"Trinidad and Tobago": "TT", "TTO": "TT", "Trinidadian": "TT",
		"Colombia": "CO", "COL": "CO", "Colombian": "CO",
		"Venezuela": "VE", "VEN": "VE", "Venezuelan": "VE",
		"Guyana": "GY", "GUY": "GY", "Guyanese": "GY",
		"Suriname": "SR", "SUR": "SR", "Surinamese": "SR",
		"Ecuador": "EC", "ECU": "EC", "Ecuadorian": "EC",
		"Peru": "PE", "PER": "PE", "Peruvian": "PE",
		"Bolivia": "BO", "BOL": "BO", "Bolivian": "BO",
		"Paraguay": "PY", "PRY": "PY", "Paraguayan": "PY",
		"Uruguay": "UY", "URY": "UY", "Uruguayan": "UY",
		"Papua New Guinea": "PG", "PNG": "PG",
		"Solomon Islands": "SB", "SLB": "SB",
		"Vanuatu": "VU", "VUT": "VU", "Ni-Vanuatu": "VU",
		"Fiji": "FJ", "FJI": "FJ", "Fijian": "FJ",
		"Kiribati": "KI", "KIR": "KI", "I-Kiribati": "KI",
		"Nauru": "NR", "NRU": "NR", "Nauruan": "NR",
		"Micronesia": "FM", "FSM": "FM", "Micronesian": "FM",
		"Marshall Islands": "MH", "MHL": "MH", "Marshallese": "MH",
		"Palau": "PW", "PLW": "PW", "Palauan": "PW",
		"Samoa": "WS", "WSM": "WS", "Samoan": "WS",
		"Tonga": "TO", "TON": "TO", "Tongan": "TO",
		"Tuvalu": "TV", "TUV": "TV", "Tuvaluan": "TV",
		"Kosovo": "XK", "XKS": "XK", "Kosovar": "XK",
		"Western Sahara": "EH", "ESH": "EH",
		"Puerto Rico": "PR", "PRI": "PR", "Puerto Rican": "PR",
		"Antarctica": "AQ", "ATA": "AQ",
		"Greenland": "GL", "GRL": "GL", "Greenlandic": "GL",
		"Reunion": "RE", "REU": "RE", "Reunionese": "RE",
		"French Guiana": "GF", "GUF": "GF",
		"French Polynesia": "PF", "PYF": "PF",
		"Saint Martin": "MF", "MAF": "MF", "St Martin": "MF",
		"Saint Pierre and Miquelon": "PM", "SPM": "PM",
		"New Caledonia": "NC", "NCL": "NC", "Nouvelle-Calédonie": "NC",
		"Wallis and Futuna": "WF", "WLF": "WF",
		"Mayotte": "YT", "MYT": "YT",
		"French Southern Territories": "TF", "ATF": "TF", "French Southern and Antarctic Lands": "TF",
		"British Virgin Islands": "VG", "VGB": "VG", "BVI": "VG",
		"Cayman Islands": "KY", "CYM": "KY", "Cayman": "KY",
		"Montserrat": "MS", "MSR": "MS",
		"Anguilla": "AI", "AIA": "AI",
		"Turks and Caicos Islands": "TC", "TCA": "TC", "Turks and Caicos": "TC",
		"Bermuda": "BM", "BMU": "BM",
		"Gibraltar": "GI", "GIB": "GI",
		"Falkland Islands": "FK", "FLK": "FK", "Malvinas": "FK",
		"Saint Helena": "SH", "SHN": "SH", "St Helena": "SH",
		"Pitcairn Islands": "PN", "PCN": "PN", "Pitcairn": "PN",
		"British Indian Ocean Territory": "IO", "IOT": "IO", "BIOT": "IO",
		"Aruba": "AW", "ABW": "AW",
		"Curaçao": "CW", "CUW": "CW", "Curacao": "CW",
		"Sint Maarten": "SX", "SXM": "SX", "Dutch Saint Martin": "SX",
		"Bonaire": "BQ", "BES": "BQ", "Caribbean Netherlands": "BQ",
		// 法属、英属、荷属地区别名
		"法属": "RE", "French Overseas": "RE", "French Territory": "RE",
		"英属": "VG", "British Overseas": "VG", "British Territory": "VG",
		"Dutch Caribbean": "AW", "Dutch Territory": "AW",
		// 群岛和地区别名
		"Caribbean": "BS", "加勒比": "BS", "Caribbean Islands": "BS",
		"Pacific": "FJ", "太平洋": "FJ", "Pacific Islands": "FJ",
		"Oceania": "AU", "大洋洲": "AU",
		"Balkans": "RS", "巴尔干": "RS", "Balkan": "RS",
		"Scandinavia": "SE", "斯堪的纳维亚": "SE", "Nordic": "SE",
		"Baltic": "EE", "波罗的海": "EE", "Baltic States": "EE",
		"Middle East": "SA", "中东": "SA",
		"Central Asia": "KZ", "中亚": "KZ",
		"Southeast Asia": "SG", "东南亚": "SG", "SEA": "SG",
		"East Asia": "JP", "东亚": "JP",
		"South Asia": "IN", "南亚": "IN",
		"West Africa": "NG", "西非": "NG",
		"East Africa": "KE", "东非": "KE",
		"Southern Africa": "ZA", "南部非洲": "ZA",
		"North Africa": "EG", "北非": "EG",
		"Central Africa": "CD", "中部非洲": "CD",
		"Central America": "CR", "中美洲": "CR",
		"South America": "BR", "南美洲": "BR",
		"North America": "US", "北美洲": "US",
		"West Indies": "JM", "西印度群岛": "JM",
		"Polynesia": "WS", "波利尼西亚": "WS",
		"Melanesia": "PG", "美拉尼西亚": "PG",
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
