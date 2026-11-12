package css

import "strings"

func words(s string) []string {
	var out []string
	for _, v := range Split(s, ' ') {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func expandDeclaration(name, value string, important bool) []Declaration {
	result := []Declaration{{name, value, important}}
	parts := words(value)
	add := func(k, v string) { result = append(result, Declaration{k, v, important}) }
	for _, property := range []string{"padding", "margin"} {
		if name == property+"-inline" || name == property+"-block" {
			if len(parts) < 1 || len(parts) > 2 {
				return result
			}
			a, b := "left", "right"
			if strings.HasSuffix(name, "block") {
				a, b = "top", "bottom"
			}
			end := parts[0]
			if len(parts) > 1 {
				end = parts[1]
			}
			add(property+"-"+a, parts[0])
			add(property+"-"+b, end)
		}
	}
	if (name == "inset" || name == "padding" || name == "margin") && len(parts) > 0 && len(parts) <= 4 {
		v := [4]string{parts[0], parts[0], parts[0], parts[0]}
		if len(parts) > 1 {
			v[1], v[3] = parts[1], parts[1]
		}
		if len(parts) > 2 {
			v[2] = parts[2]
		}
		if len(parts) > 3 {
			v[3] = parts[3]
		}
		for i, key := range []string{"top", "right", "bottom", "left"} {
			if name != "inset" {
				key = name + "-" + key
			}
			add(key, v[i])
		}
	}
	if name == "place-items" && len(parts) > 0 {
		add("align-items", parts[0])
		end := parts[0]
		if len(parts) > 1 {
			end = parts[1]
		}
		add("justify-items", end)
	}
	return result
}
