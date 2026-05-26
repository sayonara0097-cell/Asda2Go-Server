package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type gmClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginResponse struct {
	OK       bool          `json:"ok"`
	Session  session       `json:"session"`
	Commands []commandInfo `json:"commands"`
	Message  string        `json:"message,omitempty"`
}

type session struct {
	Token     string    `json:"token"`
	AccountID uint32    `json:"accountId"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type commandInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Args        []string `json:"args,omitempty"`
}

type announceRequest struct {
	Message string `json:"message"`
}

type commandRequest struct {
	Action         string            `json:"action"`
	TargetServerID string            `json:"targetServerId,omitempty"`
	Args           map[string]string `json:"args,omitempty"`
}

type commandResponse struct {
	Accepted bool     `json:"accepted"`
	Sent     int      `json:"sent"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

type gameServerStatus struct {
	ServerID      string               `json:"serverId"`
	Channel       byte                 `json:"channel"`
	IP            string               `json:"ip"`
	Port          uint16               `json:"port"`
	PlayerCount   int                  `json:"playerCount"`
	LastHeartbeat time.Time            `json:"lastHeartbeat"`
	AgeSeconds    int64                `json:"ageSeconds"`
	Players       []onlinePlayerStatus `json:"players,omitempty"`
}

type onlinePlayerStatus struct {
	ServerID  string    `json:"serverId"`
	Channel   byte      `json:"channel"`
	AccountID uint32    `json:"accountId"`
	SessionID int16     `json:"sessionId"`
	Character string    `json:"character"`
	Level     byte      `json:"level"`
	MapID     uint16    `json:"mapId"`
	X         float32   `json:"x"`
	Y         float32   `json:"y"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type itemTemplateInfo struct {
	ItemID             int    `json:"ItemID"`
	Name               string `json:"Name"`
	Kind               byte   `json:"Kind"`
	Category           int    `json:"Category"`
	InventoryType      byte   `json:"InventoryType"`
	EquipmentSlot      int16  `json:"EquipmentSlot"`
	RequiredLevel      byte   `json:"RequiredLevel"`
	RequiredProfession byte   `json:"RequiredProfession"`
	MaxStack           int    `json:"MaxStack"`
	IsStackable        bool   `json:"IsStackable"`
}

func main() {
	server := flag.String("server", "http://127.0.0.1:7000", "relay HTTP base URL")
	user := flag.String("user", "", "GM account name")
	password := flag.String("password", "", "GM account password")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)
	if strings.TrimSpace(*user) == "" {
		*user = prompt(reader, "Account: ")
	}
	if *password == "" {
		*password = prompt(reader, "Password: ")
	}

	client := &gmClient{
		baseURL: strings.TrimRight(*server, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
	login, err := client.login(*user, *password)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}
	client.token = login.Session.Token

	fmt.Printf("Logged in as %s (%s). Session expires %s.\n", login.Session.Name, login.Session.Role, login.Session.ExpiresAt.Format(time.RFC3339))
	printCommands(login.Commands)
	runMenu(reader, client)
}

func runMenu(reader *bufio.Reader, client *gmClient) {
	for {
		fmt.Println()
		fmt.Println("GM Tool")
		fmt.Println("1. World announcement")
		fmt.Println("2. Online players")
		fmt.Println("3. Add EXP to player")
		fmt.Println("4. Set player level")
		fmt.Println("5. Set player profession")
		fmt.Println("6. Heal player")
		fmt.Println("7. Damage player")
		fmt.Println("8. Teleport player")
		fmt.Println("9. Kill monster")
		fmt.Println("10. Summon monster")
		fmt.Println("11. Summon monster near player")
		fmt.Println("12. Reload monster spawns")
		fmt.Println("13. Send packet to player")
		fmt.Println("14. List game servers")
		fmt.Println("15. Set player speed")
		fmt.Println("16. Add gold to player")
		fmt.Println("17. Give item to player")
		fmt.Println("Command: set_profession <character> <class 1-9> <level 1-4>")
		fmt.Println("Command: set_speed <character> <x1-x20>")
		fmt.Println("Command: add_gold <character> <amount>")
		fmt.Println("Command: give_item <character> <itemId> [amount]")
		fmt.Println("0. Exit")
		choice := prompt(reader, "> ")

		if handleTextCommand(client, choice) {
			continue
		}

		switch choice {
		case "1":
			message := prompt(reader, "Message: ")
			var out map[string]any
			err := client.do(http.MethodPost, "/gm/announce", announceRequest{Message: message}, &out)
			printResult("announcement", err, out)
		case "2":
			players, err := fetchPlayers(client)
			if err != nil {
				fmt.Printf("Failed: %v\n", err)
				continue
			}
			printPlayers(players)
		case "3":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"amount":    prompt(reader, "EXP amount: "),
			}
			sendCommandTo(client, "add_exp", player.ServerID, args)
		case "4":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"level":     promptDefault(reader, "Level", fmt.Sprintf("%d", player.Level)),
			}
			sendCommandTo(client, "set_level", player.ServerID, args)
		case "5":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			fmt.Println("Classes: 1 OHS, 2 Spear, 3 THS, 4 Crossbow, 5 Bow, 6 Balista, 7 AttackMage, 8 SupportMage, 9 HealMage")
			args := map[string]string{
				"character": player.Character,
				"class":     prompt(reader, "Class: "),
				"level":     promptDefault(reader, "Real profession level", "1"),
			}
			sendCommandTo(client, "set_profession", player.ServerID, args)
		case "6":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"amount":    promptDefault(reader, "Heal amount (0 = full)", "0"),
			}
			sendCommandTo(client, "heal_player", player.ServerID, args)
		case "7":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"amount":    promptDefault(reader, "Damage amount", "10"),
			}
			sendCommandTo(client, "damage_player", player.ServerID, args)
		case "8":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"map":       promptDefault(reader, "Map id", fmt.Sprintf("%d", player.MapID)),
				"x":         promptDefault(reader, "Local X", fmt.Sprintf("%.0f", player.X)),
				"y":         promptDefault(reader, "Local Y", fmt.Sprintf("%.0f", player.Y)),
			}
			sendCommandTo(client, "teleport", player.ServerID, args)
		case "9":
			args := promptMonsterKillArgs(reader)
			sendCommand(reader, client, "kill_monster", args)
		case "10":
			args := map[string]string{
				"monsterId": prompt(reader, "Monster id: "),
				"map":       prompt(reader, "Map id: "),
				"x":         prompt(reader, "Local X: "),
				"y":         prompt(reader, "Local Y: "),
			}
			sendCommand(reader, client, "summon_monster", args)
		case "11":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"monsterId": prompt(reader, "Monster id: "),
				"distance":  promptDefault(reader, "Distance", "2"),
			}
			sendCommandTo(client, "summon_monster_near_player", player.ServerID, args)
		case "12":
			sendCommand(reader, client, "reload_monster_spawns", map[string]string{})
		case "13":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character":  player.Character,
				"opcode":     prompt(reader, "Opcode: "),
				"payloadHex": prompt(reader, "Payload hex: "),
			}
			sendCommandTo(client, "send_packet", player.ServerID, args)
		case "14":
			var servers []gameServerStatus
			err := client.do(http.MethodGet, "/gm/gameservers", nil, &servers)
			if err != nil {
				fmt.Printf("Failed: %v\n", err)
				continue
			}
			if len(servers) == 0 {
				fmt.Println("No connected game servers.")
				continue
			}
			for _, server := range servers {
				fmt.Printf("%s channel=%d endpoint=%s:%d players=%d heartbeat=%ds\n",
					server.ServerID, server.Channel, server.IP, server.Port, server.PlayerCount, server.AgeSeconds)
			}
		case "15":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character":  player.Character,
				"multiplier": promptDefault(reader, "Speed multiplier (x1-x20)", "x2"),
			}
			sendCommandTo(client, "set_speed", player.ServerID, args)
		case "16":
			player, ok := selectPlayer(reader, client)
			if !ok {
				continue
			}
			args := map[string]string{
				"character": player.Character,
				"amount":    prompt(reader, "Gold amount: "),
			}
			sendCommandTo(client, "add_gold", player.ServerID, args)
		case "17":
			giveItemFlow(reader, client)
		case "0", "q", "quit", "exit":
			return
		default:
			fmt.Println("Unknown choice.")
		}
	}
}

func handleTextCommand(client *gmClient, input string) bool {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "set_profession":
		if len(fields) != 4 {
			fmt.Println("Usage: set_profession <character> <class 1-9> <level 1-4>")
			return true
		}
		args := map[string]string{
			"character": fields[1],
			"class":     fields[2],
			"level":     fields[3],
		}
		sendCommandTo(client, "set_profession", "", args)
		return true
	case "set_speed":
		if len(fields) != 3 {
			fmt.Println("Usage: set_speed <character> <x1-x20>")
			return true
		}
		args := map[string]string{
			"character":  fields[1],
			"multiplier": fields[2],
		}
		sendCommandTo(client, "set_speed", "", args)
		return true
	case "add_gold":
		if len(fields) != 3 {
			fmt.Println("Usage: add_gold <character> <amount>")
			return true
		}
		args := map[string]string{
			"character": fields[1],
			"amount":    fields[2],
		}
		sendCommandTo(client, "add_gold", "", args)
		return true
	case "give_item":
		if len(fields) != 3 && len(fields) != 4 {
			fmt.Println("Usage: give_item <character> <itemId> [amount]")
			return true
		}
		amount := "1"
		if len(fields) == 4 {
			amount = fields[3]
		}
		args := map[string]string{
			"character": fields[1],
			"itemId":    fields[2],
			"amount":    amount,
		}
		sendCommandTo(client, "give_item", "", args)
		return true
	default:
		return false
	}
}

func giveItemFlow(reader *bufio.Reader, client *gmClient) {
	player, ok := selectPlayer(reader, client)
	if !ok {
		return
	}
	items, err := fetchItemTemplates(client)
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	if len(items) == 0 {
		fmt.Println("No item templates found in DB.")
		return
	}

	category := promptItemCategory(reader)
	search := strings.ToLower(strings.TrimSpace(prompt(reader, "Search name/id (blank = all): ")))
	filtered := filterItemTemplates(items, category, search)
	if len(filtered) == 0 {
		fmt.Println("No items matched that filter.")
		return
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].RequiredLevel == filtered[j].RequiredLevel {
			return filtered[i].ItemID < filtered[j].ItemID
		}
		return filtered[i].RequiredLevel < filtered[j].RequiredLevel
	})

	item, ok := selectItemTemplate(reader, filtered)
	if !ok {
		return
	}
	amount := "1"
	if item.IsStackable || item.MaxStack > 1 {
		amount = promptDefault(reader, "Amount", "1")
	}
	args := map[string]string{
		"character": player.Character,
		"itemId":    fmt.Sprintf("%d", item.ItemID),
		"amount":    amount,
	}
	sendCommandTo(client, "give_item", player.ServerID, args)
}

func fetchItemTemplates(client *gmClient) ([]itemTemplateInfo, error) {
	var items []itemTemplateInfo
	err := client.do(http.MethodGet, "/gm/items", nil, &items)
	return items, err
}

func promptItemCategory(reader *bufio.Reader) string {
	fmt.Println("Item categories:")
	fmt.Println("1. All items")
	fmt.Println("2. Weapons")
	fmt.Println("3. Armor")
	fmt.Println("4. Rings")
	fmt.Println("5. Accessories")
	fmt.Println("6. Avatar")
	fmt.Println("7. Consumables / tools")
	fmt.Println("8. Materials")
	fmt.Println("9. Sowels")
	fmt.Println("10. Recipes")
	fmt.Println("11. Mounts / vehicles")
	choice := promptDefault(reader, "Category", "1")
	switch choice {
	case "2":
		return "weapons"
	case "3":
		return "armor"
	case "4":
		return "rings"
	case "5":
		return "accessories"
	case "6":
		return "avatar"
	case "7":
		return "consumables"
	case "8":
		return "materials"
	case "9":
		return "sowels"
	case "10":
		return "recipes"
	case "11":
		return "mounts"
	default:
		return "all"
	}
}

func filterItemTemplates(items []itemTemplateInfo, category string, search string) []itemTemplateInfo {
	out := make([]itemTemplateInfo, 0, len(items))
	for _, item := range items {
		if item.ItemID <= 0 {
			continue
		}
		if !itemMatchesCategory(item, category) {
			continue
		}
		if search != "" && !itemMatchesSearch(item, search) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func itemMatchesSearch(item itemTemplateInfo, search string) bool {
	if strings.Contains(strings.ToLower(item.Name), search) {
		return true
	}
	return strings.Contains(strconv.Itoa(item.ItemID), search)
}

func itemMatchesCategory(item itemTemplateInfo, category string) bool {
	switch category {
	case "weapons":
		return item.Kind == 3
	case "armor":
		return item.Kind == 4
	case "rings":
		return item.EquipmentSlot == 5 || item.EquipmentSlot == 6
	case "accessories":
		return item.Kind == 6
	case "avatar":
		return item.Kind == 5
	case "consumables":
		return item.Kind == 2
	case "materials":
		return item.Kind == 1
	case "sowels":
		return item.Kind == 7
	case "recipes":
		return item.Kind == 8
	case "mounts":
		name := strings.ToLower(item.Name)
		return strings.Contains(name, "mount") ||
			strings.Contains(name, "vehicle") ||
			strings.Contains(name, "veiche") ||
			strings.Contains(name, "ride") ||
			strings.Contains(name, "مركب")
	default:
		return true
	}
}

func selectItemTemplate(reader *bufio.Reader, items []itemTemplateInfo) (itemTemplateInfo, bool) {
	const pageSize = 25
	page := 0
	for {
		start := page * pageSize
		if start >= len(items) {
			page = 0
			start = 0
		}
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		fmt.Printf("Items %d-%d of %d:\n", start+1, end, len(items))
		for i := start; i < end; i++ {
			item := items[i]
			fmt.Printf("%d. id=%d name=%q kind=%s cat=%d slot=%d lvl=%d stack=%t\n",
				i-start+1, item.ItemID, item.Name, itemKindName(item.Kind), item.Category, item.EquipmentSlot, item.RequiredLevel, item.IsStackable)
		}
		choice := strings.ToLower(prompt(reader, "Select number, item id, n=next, p=prev, q=cancel: "))
		switch choice {
		case "", "q", "quit", "cancel":
			return itemTemplateInfo{}, false
		case "n", "next":
			if end < len(items) {
				page++
			}
			continue
		case "p", "prev":
			if page > 0 {
				page--
			}
			continue
		}
		value, err := strconv.Atoi(choice)
		if err != nil {
			fmt.Println("Invalid choice.")
			continue
		}
		if value >= 1 && value <= end-start {
			return items[start+value-1], true
		}
		for _, item := range items {
			if item.ItemID == value {
				return item, true
			}
		}
		fmt.Println("Item not found.")
	}
}

func itemKindName(kind byte) string {
	switch kind {
	case 1:
		return "material"
	case 2:
		return "consumable"
	case 3:
		return "weapon"
	case 4:
		return "armor"
	case 5:
		return "avatar"
	case 6:
		return "accessory"
	case 7:
		return "sowel"
	case 8:
		return "recipe"
	case 9:
		return "currency"
	default:
		return "unknown"
	}
}

func promptMonsterKillArgs(reader *bufio.Reader) map[string]string {
	fmt.Println("Kill monster by:")
	fmt.Println("1. Session id")
	fmt.Println("2. Spawn id")
	fmt.Println("3. Monster id / entry id")
	choice := promptDefault(reader, "Type", "1")
	switch choice {
	case "2":
		return map[string]string{"spawnId": prompt(reader, "Spawn id: ")}
	case "3":
		return map[string]string{"monsterId": prompt(reader, "Monster id: ")}
	default:
		return map[string]string{"monsterSessionId": prompt(reader, "Monster session id: ")}
	}
}

func sendCommand(reader *bufio.Reader, client *gmClient, action string, args map[string]string) {
	target := prompt(reader, "Target server id (blank = all): ")
	sendCommandTo(client, action, target, args)
}

func sendCommandTo(client *gmClient, action string, target string, args map[string]string) {
	var out commandResponse
	err := client.do(http.MethodPost, "/gm/command", commandRequest{
		Action:         action,
		TargetServerID: target,
		Args:           args,
	}, &out)
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	fmt.Printf("Queued %s: sent=%d failed=%d accepted=%v\n", action, out.Sent, out.Failed, out.Accepted)
	for _, msg := range out.Errors {
		fmt.Printf("  error: %s\n", msg)
	}
}

func fetchPlayers(client *gmClient) ([]onlinePlayerStatus, error) {
	var players []onlinePlayerStatus
	err := client.do(http.MethodGet, "/gm/players", nil, &players)
	return players, err
}

func printPlayers(players []onlinePlayerStatus) {
	if len(players) == 0 {
		fmt.Println("No online players.")
		return
	}
	for i, player := range players {
		fmt.Printf("%d. %s server=%s channel=%d account=%d session=%d level=%d map=%d x=%.1f y=%.1f\n",
			i+1,
			player.Character,
			player.ServerID,
			player.Channel,
			player.AccountID,
			player.SessionID,
			player.Level,
			player.MapID,
			player.X,
			player.Y,
		)
	}
}

func selectPlayer(reader *bufio.Reader, client *gmClient) (onlinePlayerStatus, bool) {
	players, err := fetchPlayers(client)
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return onlinePlayerStatus{}, false
	}
	if len(players) == 0 {
		fmt.Println("No online players.")
		return onlinePlayerStatus{}, false
	}
	printPlayers(players)
	choice := prompt(reader, "Select player number or name: ")
	if choice == "" {
		return onlinePlayerStatus{}, false
	}
	for i, player := range players {
		if choice == fmt.Sprintf("%d", i+1) || strings.EqualFold(choice, player.Character) {
			return player, true
		}
	}
	fmt.Println("Player not found.")
	return onlinePlayerStatus{}, false
}

func (c *gmClient) login(name string, password string) (loginResponse, error) {
	var out loginResponse
	err := c.do(http.MethodPost, "/gm/login", loginRequest{Name: name, Password: password}, &out)
	return out, err
}

func (c *gmClient) do(method string, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func promptDefault(reader *bufio.Reader, label string, defaultValue string) string {
	value := prompt(reader, fmt.Sprintf("%s [%s]: ", label, defaultValue))
	if value == "" {
		return defaultValue
	}
	return value
}

func printCommands(commands []commandInfo) {
	if len(commands) == 0 {
		return
	}
	fmt.Println("Available commands:")
	for _, cmd := range commands {
		args := ""
		if len(cmd.Args) > 0 {
			args = " (" + strings.Join(cmd.Args, ", ") + ")"
		}
		fmt.Printf("- %s%s: %s\n", cmd.Name, args, cmd.Description)
	}
}

func printResult(name string, err error, out map[string]any) {
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Printf("%s ok:\n%s\n", name, data)
}
