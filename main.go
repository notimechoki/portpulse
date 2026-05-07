package main

import (
	"flag"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

type PortResult struct {
	Port       int
	Status     string
	Service    string
	DurationMs int64
	Error      string
}

var commonServices = map[int]string{
	20:    "ftp-data",
	21:    "ftp",
	22:    "ssh",
	25:    "smtp",
	53:    "dns",
	80:    "http",
	110:   "pop3",
	143:   "imap",
	443:   "https",
	3306:  "mysql",
	5432:  "postgresql",
	6379:  "redis",
	8000:  "dev-http",
	8080:  "dev-http",
	27017: "mongodb",
}

var portProfiles = map[string]string{
	"common": "22,80,443",
	"web":    "80,443,3000,5000,5173,8000,8080",
	"db":     "3306,5432,6379,27017",
	"dev":    "22,80,443,3000,5000,5173,5432,6379,8000,8080",
}

func main() {
	host := flag.String("host", "localhost", "Host to scan")
	profileInput := flag.String("profile", "common", "Port profile to scan. Available: common, web, db, dev")
	portsInput := flag.String("ports", "", "Ports to scan. Example: 22,80,443 or 20-100")
	timeoutInput := flag.Duration("timeout", 800*time.Millisecond, "Connection timeout. Example: 500ms, 1s")

	flag.Parse()

	ports, targetLabel, err := resolvePorts(*portsInput, *profileInput)

	if err != nil {
		fmt.Println(paint("Error:", colorRed, colorBold), err)
		return
	}

	start := time.Now()

	results := scanPorts(*host, ports, *timeoutInput)

	scanDuration := time.Since(start)

	printReport(*host, targetLabel, *timeoutInput, scanDuration, results)
}

func resolvePorts(portsInput string, profileInput string) ([]int, string, error) {
	portsInput = strings.TrimSpace(portsInput)
	profileInput = strings.ToLower(strings.TrimSpace(profileInput))

	if portsInput != "" {
		ports, err := parsePorts(portsInput)

		if err != nil {
			return nil, "", err
		}

		return ports, "custom ports: " + portsInput, nil
	}

	if profileInput == "" {
		profileInput = "common"
	}

	profilePorts, exists := portProfiles[profileInput]

	if !exists {
		return nil, "", fmt.Errorf("unknown profile: %s. Available profiles: common, web, db, dev", profileInput)
	}

	ports, err := parsePorts(profilePorts)

	if err != nil {
		return nil, "", err
	}

	return ports, "profile: " + profileInput + " (" + profilePorts + ")", nil
}

func parsePorts(input string) ([]int, error) {
	var ports []int

	parts := strings.Split(input, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")

			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))

			if err != nil {
				return nil, fmt.Errorf("invalid start port: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))

			if err != nil {
				return nil, fmt.Errorf("invalid end port: %s", rangeParts[1])
			}

			if start > end {
				return nil, fmt.Errorf("invalid range: start port is greater than end port")
			}

			for port := start; port <= end; port++ {
				if isValidPort(port) {
					ports = append(ports, port)
				}
			}

			continue
		}

		port, err := strconv.Atoi(part)

		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", part)
		}

		if !isValidPort(port) {
			return nil, fmt.Errorf("port out of range: %d", port)
		}

		ports = append(ports, port)
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("no valid ports provided")
	}

	ports = uniqueInts(ports)

	sort.Ints(ports)

	return ports, nil
}

func uniqueInts(numbers []int) []int {
	seen := map[int]bool{}
	result := []int{}

	for _, number := range numbers {
		if !seen[number] {
			seen[number] = true
			result = append(result, number)
		}
	}

	return result
}

func isValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func scanPorts(host string, ports []int, timeout time.Duration) []PortResult {
	resultsChannel := make(chan PortResult)

	for _, port := range ports {
		go func(port int) {
			result := scanPort(host, port, timeout)
			resultsChannel <- result
		}(port)
	}

	results := []PortResult{}

	for i := 0; i < len(ports); i++ {
		result := <-resultsChannel
		results = append(results, result)
	}

	sort.Slice(results, func(i int, j int) bool {
		return results[i].Port < results[j].Port
	})

	return results
}

func scanPort(host string, port int, timeout time.Duration) PortResult {
	address := fmt.Sprintf("%s:%d", host, port)

	start := time.Now()

	connection, err := net.DialTimeout("tcp", address, timeout)

	duration := time.Since(start).Milliseconds()

	service := getServiceName(port)

	if err != nil {
		return PortResult{
			Port:       port,
			Status:     "closed",
			Service:    service,
			DurationMs: 0,
			Error:      err.Error(),
		}
	}

	connection.Close()

	return PortResult{
		Port:       port,
		Status:     "open",
		Service:    service,
		DurationMs: duration,
		Error:      "",
	}
}

func getServiceName(port int) string {
	service, exists := commonServices[port]

	if exists {
		return service
	}

	return "unknown"
}

func printReport(host string, targetLabel string, timeout time.Duration, scanDuration time.Duration, results []PortResult) {
	openCount := 0
	closedCount := 0

	for _, result := range results {
		if result.Status == "open" {
			openCount++
		} else {
			closedCount++
		}
	}

	fmt.Println()
	fmt.Println(paint("╭──────────────────────────────────────────────╮", colorCyan, colorBold))
	fmt.Println(paint("│                  PortPulse                   │", colorCyan, colorBold))
	fmt.Println(paint("╰──────────────────────────────────────────────╯", colorCyan, colorBold))
	fmt.Println()

	fmt.Println(paint("Target", colorBlue, colorBold))
	fmt.Println("  Host:     ", paint(host, colorBold))
	fmt.Println("  Ports:    ", targetLabel)
	fmt.Println("  Timeout:  ", timeout)
	fmt.Println("  Duration: ", scanDuration.Round(time.Millisecond))
	fmt.Println()

	fmt.Printf("%-8s %-10s %-14s %-8s\n",
		paint("PORT", colorGray, colorBold),
		paint("STATUS", colorGray, colorBold),
		paint("SERVICE", colorGray, colorBold),
		paint("TIME", colorGray, colorBold),
	)

	fmt.Println(paint("──────────────────────────────────────────────", colorGray))

	for _, result := range results {
		statusText := formatStatus(result.Status)
		timeText := "-"

		if result.Status == "open" {
			timeText = fmt.Sprintf("%dms", result.DurationMs)
		}

		fmt.Printf("%-8d %s %-14s %-8s\n",
			result.Port,
			padColored(statusText, strings.ToUpper(result.Status), 10),
			result.Service,
			timeText,
		)
	}

	fmt.Println(paint("──────────────────────────────────────────────", colorGray))
	fmt.Println()

	fmt.Println(paint("Summary", colorBlue, colorBold))
	fmt.Println("  Open:   ", paint(strconv.Itoa(openCount), colorGreen, colorBold))
	fmt.Println("  Closed: ", paint(strconv.Itoa(closedCount), colorRed, colorBold))
	fmt.Println("  Total:  ", len(results))
	fmt.Println()
}

func formatStatus(status string) string {
	switch status {
	case "open":
		return paint("OPEN", colorGreen, colorBold)
	case "closed":
		return paint("CLOSED", colorRed, colorBold)
	default:
		return paint(strings.ToUpper(status), colorYellow, colorBold)
	}
}

func padColored(coloredText string, plainText string, width int) string {
	padding := width - len(plainText)

	if padding < 0 {
		padding = 0
	}

	return coloredText + strings.Repeat(" ", padding)
}

func paint(text string, styles ...string) string {
	return strings.Join(styles, "") + text + colorReset
}
