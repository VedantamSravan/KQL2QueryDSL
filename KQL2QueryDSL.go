package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
)

func ConvertKQL2DSL(kql string) (map[string]interface{}, error) {
	kql = normalizeWhitespace(kql)

	if strings.TrimSpace(kql) == "" {
		return map[string]interface{}{"match_all": map[string]interface{}{}}, nil
	}
	return parseExpression(kql)
}

// Normalize whitespace to handle multiline queries
func normalizeWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

func parseExpression(expr string) (map[string]interface{}, error) {
	expr = strings.TrimSpace(expr)

	for strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		inner := expr[1 : len(expr)-1]
		if balancedParentheses(inner) {
			expr = strings.TrimSpace(inner)
		} else {
			break
		}
	}

	orParts := safeSplit(expr, " or ")
	if len(orParts) > 1 {
		var should []interface{}
		for _, part := range orParts {
			q, err := parseExpression(part)
			if err != nil {
				return nil, err
			}
			should = append(should, q)
		}
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               should,
				"minimum_should_match": 1,
			},
		}, nil
	}

	andParts := safeSplit(expr, " and ")
	if len(andParts) > 1 {
		var must []interface{}
		for _, part := range andParts {
			q, err := parseExpression(part)
			if err != nil {
				return nil, err
			}
			must = append(must, q)
		}
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}, nil
	}

	// Handle NOT
	if strings.HasPrefix(expr, "not ") {
		rest := strings.TrimSpace(expr[4:])
		sub, err := parseExpression(rest)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": []interface{}{sub},
			},
		}, nil
	}

	termsPattern := regexp.MustCompile(`^([\w.@-]+):\(([^()]+)\)$`)
	if matches := termsPattern.FindStringSubmatch(expr); len(matches) == 3 {
		field := matches[1]
		vals := safeSplit(matches[2], " or ")
		var values []interface{}
		for _, v := range vals {
			values = append(values, strings.Trim(v, `" `))
		}
		return map[string]interface{}{
			"terms": map[string]interface{}{
				field: values,
			},
		}, nil
	}

	return parseTerm(expr)
}

func parseTerm(expr string) (map[string]interface{}, error) {
	expr = strings.TrimSpace(expr)

	rangeRegex := regexp.MustCompile(`^([\w.@-]+)\s*([><]=?)\s*("?[^"]+"?|\d+)$`)
	if matches := rangeRegex.FindStringSubmatch(expr); len(matches) == 4 {
		field, op, val := matches[1], matches[2], strings.Trim(matches[3], `"`)
		opMap := map[string]string{">": "gt", ">=": "gte", "<": "lt", "<=": "lte"}
		return map[string]interface{}{
			"range": map[string]interface{}{
				field: map[string]interface{}{
					opMap[op]: val,
				},
			},
		}, nil
	}

	if strings.Contains(expr, ":") {
		parts := strings.SplitN(expr, ":", 2)
		field := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `" `)

		if field == "_exists_" {
			return map[string]interface{}{
				"exists": map[string]interface{}{
					"field": value,
				},
			}, nil
		}

		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
			return map[string]interface{}{
				"match_phrase": map[string]interface{}{
					field: value,
				},
			}, nil
		}

		if strings.Contains(value, "*") {
			return map[string]interface{}{
				"wildcard": map[string]interface{}{
					field: value,
				},
			}, nil
		}

		return map[string]interface{}{
			"term": map[string]interface{}{
				field: value,
			},
		}, nil
	}

	return nil, fmt.Errorf("unrecognized expression: %s", expr)
}

func safeSplit(s, delim string) []string {
	var parts []string
	start := 0
	depth := 0
	inQuote := false

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote {
				depth--
			}
		}

		if depth == 0 && !inQuote && i+len(delim) <= len(s) &&
			s[i:i+len(delim)] == delim {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + len(delim)
			i += len(delim) - 1
		}
	}

	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}

	return parts
}

func balancedParentheses(s string) bool {
	depth := 0
	inQuote := false

	for _, r := range s {
		if r == '"' {
			inQuote = !inQuote
		} else if !inQuote {
			if r == '(' {
				depth++
			} else if r == ')' {
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}
	return depth == 0
}

func main() {
	//sample kql search strings
	kqlQueries := []string{
		`event_type:"flow" AND dest_port:80 OR event_type:"dns"`,

		`(status:500 or status:503) and source.ip:"192.168.1.1" and @timestamp > "2023-05-01"`,

		`(event.action:("login" or "sudo") and user.name:("admin" or "root")) or (source.ip:*192.168.* and not destination.port:(22 or 443 or 80)) and ((process.name:"ssh" and event.outcome:"failure" and event.count > 10) or (file.path:("/etc/passwd" or "/etc/shadow") and event.type:"change")) and not agent.type:"filebeat" and (alert.severity:(critical or high) and not alert.status:resolved)`,

		`(_exists_:error.message or log.level:("error" or "critical" or "fatal")) and (host.name:("web-server-*" or "api-server-*") or kubernetes.namespace:("production" or "staging")) and not (message:*"scheduled maintenance"* or event.reason:"HealthCheck") and @timestamp > now-7d`,

		`((service.name:("nginx" or "apache") and http.response.status_code >= 400) or (event.module:"system" and event.dataset:"syslog")) and @timestamp >= "2023-01-01" and @timestamp < "2023-02-01" and not (source.ip:"127.0.0.1" or source.ip:"::1")`,
	}

	for _, kql := range kqlQueries {

		query, err := ConvertKQL2DSL(kql)
		if err != nil {
			log.Printf("Error converting KQL to Query DSL: %v\n", err)
			continue
		}

		output, _ := json.MarshalIndent(query, "", "  ")
		fmt.Println("Converted Elasticsearch Query DSL:")
		fmt.Println(string(output))
		fmt.Println(strings.Repeat("=", 70))
	}
}
