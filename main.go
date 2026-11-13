package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"

	"github.com/AllenDang/cimgui-go/imgui"
	"golang.org/x/sys/windows"
)

//go:embed offsets.json
var embeddedOffsets []byte

//go:embed hit.wav
var embeddedHitSound []byte

// GitHub Güncelleme Ayarları
const githubRepo = "bxiefr/mesnevi_external"

var (
	currentVersion = "dev"
	buildStamp     = "0"
)

var stampRe = regexp.MustCompile(`BuildStamp:\s*(\d+)`)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type Matrix [4][4]float32

type Vector3 struct {
	X float32
	Y float32
	Z float32
}

func (v Vector3) Dist(other Vector3) float32 {
	return float32(math.Abs(float64(v.X-other.X)) + math.Abs(float64(v.Y-other.Y)) + math.Abs(float64(v.Z-other.Z)))
}

type Vector2 struct {
	X float32
	Y float32
}

type Rectangle struct {
	Top    float32
	Left   float32
	Right  float32
	Bottom float32
}

type Entity struct {
	Health   int32
	Armor    int32
	Team     int32
	Name     string
	Position Vector2
	Bones    map[string]Vector2
	HeadPos  Vector3
	Distance float32
	Rect     Rectangle
}

type Offset struct {
	DwViewMatrix           uintptr `json:"dwViewMatrix"`
	DwLocalPlayerPawn      uintptr `json:"dwLocalPlayerPawn"`
	DwEntityList           uintptr `json:"dwEntityList"`
	DwPlantedC4            uintptr `json:"dwPlantedC4"`
	DwGlobalVars           uintptr `json:"dwGlobalVars"`
	M_hPlayerPawn          uintptr `json:"m_hPlayerPawn"`
	M_iHealth              uintptr `json:"m_iHealth"`
	M_lifeState            uintptr `json:"m_lifeState"`
	M_iTeamNum             uintptr `json:"m_iTeamNum"`
	M_vOldOrigin           uintptr `json:"m_vOldOrigin"`
	M_pGameSceneNode       uintptr `json:"m_pGameSceneNode"`
	M_modelState           uintptr `json:"m_modelState"`
	M_boneArray            uintptr `json:"m_boneArray"`
	M_nodeToWorld          uintptr `json:"m_nodeToWorld"`
	M_sSanitizedPlayerName uintptr `json:"m_sSanitizedPlayerName"`
	M_ArmorValue           uintptr `json:"m_ArmorValue"`
	M_pBulletServices      uintptr `json:"m_pBulletServices"`
	M_totalHitsOnServer    uintptr `json:"m_totalHitsOnServer"`
	M_flC4Blow             uintptr `json:"m_flC4Blow"`
}

type Config struct {
	TeamCheck           bool `json:"teamCheck"`
	HeadCircle          bool `json:"headCircle"`
	SkeletonRendering   bool `json:"skeletonRendering"`
	BoxRendering        bool `json:"boxRendering"`
	NameRendering       bool `json:"nameRendering"`
	HealthBarRendering  bool `json:"healthBarRendering"`
	ArmorBarRendering   bool `json:"armorBarRendering"`
	HealthTextRendering bool `json:"healthTextRendering"`
	DistanceRendering   bool `json:"distanceRendering"`
	HitmarkerRendering  bool `json:"hitmarkerRendering"`
	HitSoundRendering   bool `json:"hitSoundRendering"`
	BombTimerRendering  bool `json:"bombTimerRendering"`
	SkeletonColorIndex  int  `json:"skeletonColorIndex"`
}

var (
	cachedScreenWidth  float32
	cachedScreenHeight float32
	user32             = windows.NewLazySystemDLL("user32.dll")
	getForegroundWindow  = user32.NewProc("GetForegroundWindow")
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	setPriorityClass   = kernel32.NewProc("SetPriorityClass")
	winmm              = windows.NewLazySystemDLL("winmm.dll")
	getSystemMetrics   = user32.NewProc("GetSystemMetrics")
	showCursor         = user32.NewProc("ShowCursor")
	GetAsyncKeyState   = user32.NewProc("GetAsyncKeyState")
	beepProc           = kernel32.NewProc("Beep")
	playSoundProc      = winmm.NewProc("PlaySoundW")
)

const (
	SND_FILENAME  = 0x00020000
	SND_ASYNC     = 0x0001
	SND_NODEFAULT = 0x0002
)

var (
	teamCheck           bool    = true
	headCircle          bool    = true
	skeletonRendering   bool    = true
	boxRendering        bool    = true
	nameRendering       bool    = true
	healthBarRendering  bool    = true
	armorBarRendering   bool    = true
	healthTextRendering bool    = true
	distanceRendering   bool    = true
	hitmarkerRendering  bool    = true
	hitSoundRendering   bool    = true
	bombTimerRendering  bool    = true
	menuOpen            bool    = true
	frameDelay          uint32  = 15
	currentHue          float64 = 0

	lastHits      int32 = -1
	lastHitTime   time.Time
	cachedHitPath string

	lastPlantedC4 uintptr
	bombPlantTime time.Time
)

var (
	skeletonColorIndex int
	colorNames         = []string{"MAVİ", "YEŞİL", "KIRMIZI", "SARI", "MOR", "TURKUAZ"}
)

func init() {
	runtime.LockOSThread()
	rand.Seed(time.Now().UnixNano())
	debug.SetGCPercent(200)
	cachedHitPath = extractEmbeddedHitSound()
	loadConfig()
}

func checkForUpdates() {
	if currentVersion == "dev" {
		fmt.Println("[*] Development surumu aktif. Guncelleme kontrolu atlandi.")
		return
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", githubRepo, currentVersion)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		fmt.Println("[-] Guncelleme sunucusuna baglanilamadi:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("[-] Guncelleme kontrol basarisiz, HTTP: %d\n", resp.StatusCode)
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("[-] Release verisi cozumlenemedi:", err)
		return
	}

	m := stampRe.FindStringSubmatch(release.Body)
	if m == nil {
		fmt.Println("[-] Release aciklamasinda BuildStamp bulunamadi.")
		return
	}

	remoteStamp, err1 := strconv.ParseInt(m[1], 10, 64)
	localStamp, err2 := strconv.ParseInt(buildStamp, 10, 64)
	if err1 != nil || err2 != nil {
		fmt.Println("[-] BuildStamp donusum hatasi.")
		return
	}

	fmt.Printf("[DEBUG] Remote BuildStamp: %d (%s)\n", remoteStamp, time.Unix(remoteStamp, 0).Format("2006-01-02 15:04:05"))
	fmt.Printf("[DEBUG] Local  BuildStamp: %d (%s)\n", localStamp, time.Unix(localStamp, 0).Format("2006-01-02 15:04:05"))

	if remoteStamp <= localStamp {
		fmt.Println("[*] Surum guncel. Baslatiliyor...")
		return
	}

	fmt.Printf("\n[!] Yeni build bulundu (%s). Guncellemek ister misin? [Y/N]: ", time.Unix(remoteStamp, 0).Format("2006-01-02 15:04"))

	var choice string
	_, _ = fmt.Scanln(&choice)
	choice = strings.TrimSpace(strings.ToUpper(choice))

	if choice != "Y" && choice != "EVET" {
		fmt.Println("[*] Guncelleme atlandi. Hile baslatiliyor...")
		return
	}

	for _, asset := range release.Assets {
		if asset.Name == "mesnevi.exe" {
			fmt.Println("[*] Guncelleme indiriliyor...")
			downloadAndUpdate(asset.BrowserDownloadURL)
			break
		}
	}
}

func downloadAndUpdate(downloadURL string) {
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	tmpPath := exePath + ".tmp"
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		fmt.Println("[-] Indirme hatasi:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("[-] Indirme basarisiz, HTTP:", resp.StatusCode)
		return
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()

	if err != nil {
		os.Remove(tmpPath)
		return
	}

	batchPath := filepath.Join(os.TempDir(), "update.bat")
	batchContent := fmt.Sprintf(`@echo off
timeout /t 2 /nobreak > nul
del /f /q "%s"
move /y "%s" "%s"
start "" "%s"
del "%%~f0"
`, exePath, tmpPath, exePath, exePath)

	if err := os.WriteFile(batchPath, []byte(batchContent), 0644); err != nil {
		return
	}

	cmd := exec.Command("cmd.exe", "/c", batchPath)
	if err := cmd.Start(); err != nil {
		return
	}

	os.Exit(0)
}

func getConfigPath() string {
	exeDir, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exeDir), "config.json")
}

func saveConfig() {
	cfg := Config{
		TeamCheck:           teamCheck,
		HeadCircle:          headCircle,
		SkeletonRendering:   skeletonRendering,
		BoxRendering:        boxRendering,
		NameRendering:       nameRendering,
		HealthBarRendering:  healthBarRendering,
		ArmorBarRendering:   armorBarRendering,
		HealthTextRendering: healthTextRendering,
		DistanceRendering:   distanceRendering,
		HitmarkerRendering:  hitmarkerRendering,
		HitSoundRendering:   hitSoundRendering,
		BombTimerRendering:  bombTimerRendering,
		SkeletonColorIndex:  skeletonColorIndex,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(getConfigPath(), data, 0644)
	}
}

func loadConfig() {
	filePath := getConfigPath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err == nil {
		teamCheck = cfg.TeamCheck
		headCircle = cfg.HeadCircle
		skeletonRendering = cfg.SkeletonRendering
		boxRendering = cfg.BoxRendering
		nameRendering = cfg.NameRendering
		healthBarRendering = cfg.HealthBarRendering
		armorBarRendering = cfg.ArmorBarRendering
		healthTextRendering = cfg.HealthTextRendering
		distanceRendering = cfg.DistanceRendering
		hitmarkerRendering = cfg.HitmarkerRendering
		hitSoundRendering = cfg.HitSoundRendering
		bombTimerRendering = cfg.BombTimerRendering
		skeletonColorIndex = cfg.SkeletonColorIndex
	}
}

func extractEmbeddedHitSound() string {
	tmpPath := filepath.Join(os.TempDir(), "mesnevi_hit.wav")
	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		_ = os.WriteFile(tmpPath, embeddedHitSound, 0644)
	}
	return tmpPath
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func playBeep(enabled bool) {
	if enabled {
		beepProc.Call(800, 100)
	} else {
		beepProc.Call(400, 100)
	}
}

func playHitSound() {
	if cachedHitPath == "" {
		return
	}
	soundPath, _ := windows.UTF16PtrFromString(cachedHitPath)
	playSoundProc.Call(uintptr(unsafe.Pointer(soundPath)), 0, uintptr(SND_FILENAME|SND_ASYNC|SND_NODEFAULT))
}

func logAndSleep(message string, err error) {
	log.Printf("%s: %v\n", message, err)
	time.Sleep(5 * time.Second)
}

func worldToScreen(viewMatrix Matrix, position Vector3) (float32, float32) {
	var screenX float32
	var screenY float32
	screenX = viewMatrix[0][0]*position.X + viewMatrix[0][1]*position.Y + viewMatrix[0][2]*position.Z + viewMatrix[0][3]
	screenY = viewMatrix[1][0]*position.X + viewMatrix[1][1]*position.Y + viewMatrix[1][2]*position.Z + viewMatrix[1][3]
	w := viewMatrix[3][0]*position.X + viewMatrix[3][1]*position.Y + viewMatrix[3][2]*position.Z + viewMatrix[3][3]
	if w < 0.01 {
		return -1, -1
	}
	invw := 1.0 / w
	screenX *= invw
	screenY *= invw
	x := cachedScreenWidth / 2
	y := cachedScreenHeight / 2
	x += 0.5 * screenX * cachedScreenWidth + 0.5
	y -= 0.5 * screenY * cachedScreenHeight + 0.5
	return x, y
}

func getOffsets() Offset {
	var offsets Offset
	err := json.Unmarshal(embeddedOffsets, &offsets)
	if err != nil {
		return offsets
	}
	return offsets
}

func getEntitiesInfo(procHandle windows.Handle, clientDll uintptr, screenWidth uintptr, screenHeight uintptr, offsets Offset) []Entity {
	var entityList uintptr
	var entities []Entity
	err := read(procHandle, clientDll+offsets.DwEntityList, &entityList)
	if err != nil {
		return entities
	}
	var (
		localPlayerP           uintptr
		localPlayerGameScene   uintptr
		localPlayerSceneOrigin Vector3
		localTeam              int32
		listEntry              uintptr
		gameScene              uintptr
		entityController       uintptr
		entityControllerPawn   uintptr
		entityPawn             uintptr
		entityNameAddress      uintptr
		entityBoneArray        uintptr
		entityTeam             int32
		entityHealth           int32
		entityArmor            int32
		entityLifeState        int32
		entityName             string
		sanitizedNameStr       string
		entityOrigin           Vector3
		viewMatrix             Matrix
	)
	bones := map[string]int{
		"head":        6,
		"neck_0":      5,
		"spine_1":     4,
		"spine_2":     2,
		"pelvis":      0,
		"arm_upper_L": 8,
		"arm_lower_L": 9,
		"hand_L":      10,
		"arm_upper_R": 13,
		"arm_lower_R": 14,
		"hand_R":      15,
		"leg_upper_L": 22,
		"leg_lower_L": 23,
		"ankle_L":     24,
		"leg_upper_R": 25,
		"leg_lower_R": 26,
		"ankle_R":     27,
	}
	var (
		currentBone      Vector3
		entityHead       Vector3
		entityHeadTop    Vector3
		entityHeadBottom Vector3
	)

	err = read(procHandle, clientDll+offsets.DwLocalPlayerPawn, &localPlayerP)
	if err != nil || localPlayerP == 0 {
		return entities
	}

	if hitmarkerRendering {
		var bulletServices uintptr
		var totalHits int32
		err = read(procHandle, localPlayerP+offsets.M_pBulletServices, &bulletServices)
		if err == nil && bulletServices != 0 {
			err = read(procHandle, bulletServices+offsets.M_totalHitsOnServer, &totalHits)
			if err == nil {
				if lastHits == -1 {
					lastHits = totalHits
				} else if totalHits > lastHits {
					lastHitTime = time.Now()
					if hitSoundRendering {
						go playHitSound()
					}
					lastHits = totalHits
				} else if totalHits < lastHits {
					lastHits = totalHits
				}
			}
		}
	}

	err = read(procHandle, localPlayerP+offsets.M_pGameSceneNode, &localPlayerGameScene)
	if err != nil {
		return entities
	}
	err = read(procHandle, localPlayerGameScene+offsets.M_nodeToWorld, &localPlayerSceneOrigin)
	if err != nil {
		return entities
	}
	err = read(procHandle, clientDll+offsets.DwViewMatrix, &viewMatrix)
	if err != nil {
		return entities
	}
	for i := 0; i < 64; i++ {
		var tempEntity Entity
		var entityBones map[string]Vector2 = make(map[string]Vector2)
		var sanitizedName strings.Builder

		err = read(procHandle, entityList+uintptr((8*(i&0x7FFF)>>9)+16), &listEntry)
		if err != nil {
			return entities
		}
		if listEntry == 0 {
			continue
		}

		err = read(procHandle, listEntry+uintptr(112)*uintptr(i&0x1FF), &entityController)
		if err != nil {
			return entities
		}
		if entityController == 0 {
			continue
		}

		err = read(procHandle, entityController+offsets.M_hPlayerPawn, &entityControllerPawn)
		if err != nil {
			return entities
		}
		if entityControllerPawn == 0 {
			continue
		}

		err = read(procHandle, entityList+uintptr(0x8*((entityControllerPawn&0x7FFF)>>9)+16), &listEntry)
		if err != nil {
			return entities
		}
		if listEntry == 0 {
			continue
		}

		err = read(procHandle, listEntry+uintptr(112)*uintptr(entityControllerPawn&0x1FF), &entityPawn)
		if err != nil {
			return entities
		}
		if entityPawn == 0 {
			continue
		}
		
		if entityPawn == localPlayerP {
			continue
		}

		err = read(procHandle, entityPawn+offsets.M_lifeState, &entityLifeState)
		if err != nil {
			return entities
		}
		if entityLifeState != 256 {
			continue
		}

		err = read(procHandle, entityPawn+offsets.M_iTeamNum, &entityTeam)
		if err != nil {
			return entities
		}
		if entityTeam == 0 {
			continue
		}
		if teamCheck {
			err = read(procHandle, localPlayerP+offsets.M_iTeamNum, &localTeam)
			if err != nil {
				return entities
			}
			if localTeam == entityTeam {
				continue
			}
		}

		err = read(procHandle, entityPawn+offsets.M_iHealth, &entityHealth)
		if err != nil {
			return entities
		}
		if entityHealth < 1 || entityHealth > 100 {
			continue
		}

		err = read(procHandle, entityPawn+offsets.M_ArmorValue, &entityArmor)
		if err != nil {
			entityArmor = 0
		}

		err = read(procHandle, entityController+offsets.M_sSanitizedPlayerName, &entityNameAddress)
		if err != nil {
			return entities
		}

		err = read(procHandle, entityNameAddress, &entityName)
		if err != nil {
			return entities
		}
		if entityName == "" {
			continue
		}
		for _, c := range entityName {
			if unicode.IsLetter(c) || unicode.IsDigit(c) || unicode.IsPunct(c) || unicode.IsSpace(c) {
				sanitizedName.WriteRune(c)
			}
		}
		sanitizedNameStr = sanitizedName.String()

		err = read(procHandle, entityPawn+offsets.M_pGameSceneNode, &gameScene)
		if err != nil {
			return entities
		}
		if gameScene == 0 {
			continue
		}

		err = read(procHandle, gameScene+offsets.M_modelState+offsets.M_boneArray, &entityBoneArray)
		if err != nil {
			return entities
		}
		if entityBoneArray == 0 {
			continue
		}

		err = read(procHandle, entityPawn+offsets.M_vOldOrigin, &entityOrigin)
		if err != nil {
			return entities
		}

		if skeletonRendering || headCircle {
			for boneName, boneIndex := range bones {
				if !skeletonRendering && boneName != "head" {
					continue
				}

				err = read(procHandle, entityBoneArray+uintptr(boneIndex)*32, &currentBone)
				if err != nil {
					return entities
				}
				if boneName == "head" {
					entityHead = currentBone
					if !skeletonRendering {
						break
					}
				}
				boneX, boneY := worldToScreen(viewMatrix, currentBone)
				entityBones[boneName] = Vector2{boneX, boneY}
			}
		}        
		
		entityHeadTop = Vector3{entityHead.X, entityHead.Y, entityHead.Z + 7}
		entityHeadBottom = Vector3{entityHead.X, entityHead.Y, entityHead.Z - 5}
		screenPosHeadX, screenPosHeadTopY := worldToScreen(viewMatrix, entityHeadTop)
		_, screenPosHeadBottomY := worldToScreen(viewMatrix, entityHeadBottom)
		screenPosFeetX, screenPosFeetY := worldToScreen(viewMatrix, entityOrigin)
		entityBoxTop := Vector3{entityOrigin.X, entityOrigin.Y, entityOrigin.Z + 70}
		_, screenPosBoxTop := worldToScreen(viewMatrix, entityBoxTop)
		if screenPosHeadX <= -1 || screenPosFeetY <= -1 || screenPosHeadX >= float32(screenWidth) || screenPosHeadTopY >= float32(screenHeight) {
			continue
		}
		boxHeight := screenPosFeetY - screenPosBoxTop

		tempEntity.Health = entityHealth
		tempEntity.Armor = entityArmor
		tempEntity.Team = entityTeam
		tempEntity.Name = sanitizedNameStr
		tempEntity.Distance = entityOrigin.Dist(localPlayerSceneOrigin) / 39.3700787
		tempEntity.Position = Vector2{screenPosFeetX, screenPosFeetY}
		tempEntity.Bones = entityBones
		tempEntity.HeadPos = Vector3{screenPosHeadX, screenPosHeadTopY, screenPosHeadBottomY}
		tempEntity.Rect = Rectangle{screenPosBoxTop, screenPosFeetX - boxHeight/4, screenPosFeetX + boxHeight/4, screenPosFeetY}

		entities = append(entities, tempEntity)
	}
	return entities
}

func drawBombTimerImGui(drawList *imgui.DrawList, procHandle windows.Handle, clientDll uintptr, screenWidth, screenHeight uintptr, offsets Offset) {
	if !bombTimerRendering {
		return
	}

	var plantedC4 uintptr
	err := read(procHandle, clientDll+offsets.DwPlantedC4, &plantedC4)
	if err != nil || plantedC4 == 0 {
		lastPlantedC4 = 0
		return
	}

	var c4Blow float32
	err = read(procHandle, plantedC4+offsets.M_flC4Blow, &c4Blow)
	if err != nil || c4Blow <= 0 {
		return
	}

	if plantedC4 != lastPlantedC4 {
		lastPlantedC4 = plantedC4
		bombPlantTime = time.Now()
	}

	elapsed := time.Since(bombPlantTime).Seconds()
	timeLeft := 40.0 - float32(elapsed)
	if timeLeft < 0 {
		timeLeft = 0
	}
	if timeLeft > 40.0 {
		timeLeft = 40.0
	}

	ratio := float64(timeLeft) / 40.0
	var rCol, gCol float32
	if ratio > 0.5 {
		rCol = float32(1.0 - ratio) * 2
		gCol = 1.0
	} else {
		rCol = 1.0
		gCol = float32(ratio * 2)
	}

	var gameScene uintptr
	var bombOrigin Vector3
	err = read(procHandle, plantedC4+offsets.M_pGameSceneNode, &gameScene)
	if err == nil && gameScene != 0 {
		read(procHandle, gameScene+offsets.M_nodeToWorld, &bombOrigin)
	}

	var viewMatrix Matrix
	read(procHandle, clientDll+offsets.DwViewMatrix, &viewMatrix)
	bx, by := worldToScreen(viewMatrix, bombOrigin)

	if bx > 0 && by > 0 && bx < float32(screenWidth) && by < float32(screenHeight) {
		col := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: rCol, Y: gCol, Z: 0, W: 1})
		drawList.AddCircle(imgui.Vec2{X: bx, Y: by}, 15, col)
	}
}

func drawSkeletonImGui(drawList *imgui.DrawList, col imgui.Vec4, bones map[string]Vector2) {
	c := imgui.ColorConvertFloat4ToU32(col)
	drawLine := func(b1, b2 string) {
		p1, ok1 := bones[b1]
		p2, ok2 := bones[b2]
		if ok1 && ok2 {
			drawList.AddLine(imgui.Vec2{X: p1.X, Y: p1.Y}, imgui.Vec2{X: p2.X, Y: p2.Y}, c)
		}
	}

	drawLine("head", "neck_0")
	drawLine("neck_0", "spine_1")
	drawLine("spine_1", "spine_2")
	drawLine("spine_2", "pelvis")
	drawLine("pelvis", "leg_upper_L")
	drawLine("leg_upper_L", "leg_lower_L")
	drawLine("leg_lower_L", "ankle_L")
	drawLine("pelvis", "leg_upper_R")
	drawLine("leg_upper_R", "leg_lower_R")
	drawLine("leg_lower_R", "ankle_R")
	drawLine("spine_1", "arm_upper_L")
	drawLine("arm_upper_L", "arm_lower_L")
	drawLine("arm_lower_L", "hand_L")
	drawLine("spine_1", "arm_upper_R")
	drawLine("arm_upper_R", "arm_lower_R")
	drawLine("arm_lower_R", "hand_R")
}

func drawHitmarkerImGui(drawList *imgui.DrawList, screenWidth, screenHeight uintptr) {
	centerX := float32(screenWidth / 2)
	centerY := float32(screenHeight / 2)
	size := float32(8)
	gap := float32(4)
	col := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: 1, Y: 1, Z: 1, W: 1})

	drawList.AddLine(imgui.Vec2{X: centerX - size, Y: centerY - size}, imgui.Vec2{X: centerX - gap, Y: centerY - gap}, col)
	drawList.AddLine(imgui.Vec2{X: centerX + gap, Y: centerY + gap}, imgui.Vec2{X: centerX + size, Y: centerY + size}, col)
	drawList.AddLine(imgui.Vec2{X: centerX + size, Y: centerY - size}, imgui.Vec2{X: centerX + gap, Y: centerY - gap}, col)
	drawList.AddLine(imgui.Vec2{X: centerX - gap, Y: centerY + gap}, imgui.Vec2{X: centerX - size, Y: centerY + size}, col)
}

func renderEntityInfoImGui(drawList *imgui.DrawList, themeCol imgui.Vec4, rect Rectangle, hp int32, armor int32, name string, headPos Vector3, distance float32) {
	tCol := imgui.ColorConvertFloat4ToU32(themeCol)

	if boxRendering {
		drawList.AddRect(imgui.Vec2{X: rect.Left, Y: rect.Top}, imgui.Vec2{X: rect.Right, Y: rect.Bottom}, tCol)
	}

	if headCircle {
		radius := (headPos.Z - headPos.Y) / 2
		drawList.AddCircle(imgui.Vec2{X: headPos.X, Y: headPos.Y + radius}, radius, tCol)
	}

	if healthBarRendering {
		if hp < 0 {
			hp = 0
		} else if hp > 100 {
			hp = 100
		}
		hpHeight := (rect.Bottom - rect.Top) * (float32(hp) / 100.0)
		hpTop := rect.Bottom - hpHeight
		hpCol := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: float32(100-hp) / 100.0, Y: float32(hp) / 100.0, Z: 0, W: 1})

		drawList.AddLine(imgui.Vec2{X: rect.Left - 4, Y: hpTop}, imgui.Vec2{X: rect.Left - 4, Y: rect.Bottom}, hpCol)
	}

	if armorBarRendering && armor > 0 {
		if armor > 100 {
			armor = 100
		}
		armorHeight := (rect.Bottom - rect.Top) * (float32(armor) / 100.0)
		armorTop := rect.Bottom - armorHeight
		armorCol := imgui.ColorConvertFloat4ToU32(imgui.Vec4{X: 0, Y: 0.8, Z: 1, W: 1})

		drawList.AddLine(imgui.Vec2{X: rect.Right + 4, Y: armorTop}, imgui.Vec2{X: rect.Right + 4, Y: rect.Bottom}, armorCol)
	}
}

func drawInGameMenu() {
	if !menuOpen {
		return
	}

	currentColor := colorNames[skeletonColorIndex]
	imgui.SetNextWindowSize(imgui.Vec2{X: 340, Y: 500})
	imgui.Begin("=== MESNEVI EXTERNAL v" + currentVersion + " ===")

	imgui.TextColored(imgui.Vec4{X: 0.5, Y: 0.6, Z: 0.7, W: 1}, "--- ESP AYARLARI ---")
	imgui.Checkbox("[F1] Box ESP", &boxRendering)
	imgui.Checkbox("[F2] Skeleton ESP", &skeletonRendering)
	imgui.Checkbox("[F3] Head Circle", &headCircle)
	imgui.Checkbox("[F4] Team Check", &teamCheck)

	imgui.Spacing()
	imgui.TextColored(imgui.Vec4{X: 0.5, Y: 0.6, Z: 0.7, W: 1}, "--- GORSEL & BILGI ---")
	imgui.Checkbox("[F5] Health Bar", &healthBarRendering)
	imgui.Checkbox("[F6] Armor Bar", &armorBarRendering)
	imgui.Checkbox("[F7] Player Name", &nameRendering)
	imgui.Checkbox("[F8] Distance Display", &distanceRendering)
	
	if imgui.Button("[F9] Tema: " + currentColor) {
		skeletonColorIndex = (skeletonColorIndex + 1) % len(colorNames)
		playBeep(true)
	}

	imgui.Checkbox("[F10] Hitmarker & Sound", &hitmarkerRendering)
	hitSoundRendering = hitmarkerRendering
	imgui.Checkbox("[F11] Bomb Timer", &bombTimerRendering)

	imgui.Spacing()
	imgui.TextColored(imgui.Vec4{X: 0.5, Y: 0.6, Z: 0.7, W: 1}, "--- SISTEM & KONTROL ---")
	if imgui.Button("[F12] Config Kaydet") {
		saveConfig()
		playBeep(true)
	}
	if imgui.Button("[DELETE] Panic Button (Cikis)") {
		playBeep(false)
		os.Exit(0)
	}

	imgui.End()
}

func getStatusText(status bool) string {
	if status {
		return "[ AÇIK ]"
	}
	return "[ KAPALI ]"
}

func isKeyPressed(vKey int) bool {
	ret, _, _ := GetAsyncKeyState.Call(uintptr(vKey))
	return (ret & 0x0001) != 0
}

func listenHotkeys() {
	for {
		if isKeyPressed(0x2D) { // VK_INSERT -> Menüyü Gizle/Göster
			menuOpen = !menuOpen
			playBeep(menuOpen)
		}
		if isKeyPressed(0x70) { // F1 -> Box ESP
			boxRendering = !boxRendering
			playBeep(boxRendering)
		}
		if isKeyPressed(0x71) { // F2 -> Skeleton ESP
			skeletonRendering = !skeletonRendering
			playBeep(skeletonRendering)
		}
		if isKeyPressed(0x72) { // F3 -> Head Circle
			headCircle = !headCircle
			playBeep(headCircle)
		}
		if isKeyPressed(0x73) { // F4 -> Team Check
			teamCheck = !teamCheck
			playBeep(teamCheck)
		}
		if isKeyPressed(0x74) { // F5 -> Health Bar
			healthBarRendering = !healthBarRendering
			playBeep(healthBarRendering)
		}
		if isKeyPressed(0x75) { // F6 -> Armor Bar
			armorBarRendering = !armorBarRendering
			playBeep(armorBarRendering)
		}
		if isKeyPressed(0x76) { // F7 -> Player Name
			nameRendering = !nameRendering
			playBeep(nameRendering)
		}
		if isKeyPressed(0x77) { // F8 -> Distance Display
			distanceRendering = !distanceRendering
			playBeep(distanceRendering)
		}
		if isKeyPressed(0x78) { // F9 -> ESP Tema Rengi
			skeletonColorIndex = (skeletonColorIndex + 1) % len(colorNames)
			playBeep(true)
		}
		if isKeyPressed(0x79) { // F10 -> Hitmarker & Sound
			hitmarkerRendering = !hitmarkerRendering
			hitSoundRendering = hitmarkerRendering
			playBeep(hitmarkerRendering)
		}
		if isKeyPressed(0x7A) { // F11 -> Bomb Timer
			bombTimerRendering = !bombTimerRendering
			playBeep(bombTimerRendering)
		}
		if isKeyPressed(0x7B) { // F12 -> Config Kaydet
			saveConfig()
			playBeep(true)
		}
		if isKeyPressed(0x2E) { // VK_DELETE -> PANIC BUTTON
			playBeep(false)
			os.Exit(0)
		}
		if isKeyPressed(0x23) { // END -> Normal Çıkış
			playBeep(false)
			os.Exit(0)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func main() {
	checkForUpdates()

	setPriorityClass.Call(uintptr(windows.CurrentProcess()), 0x00008000)
	go listenHotkeys()

	screenWidth, _, _ := getSystemMetrics.Call(0)
	screenHeight, _, _ := getSystemMetrics.Call(1)

	cachedScreenWidth = float32(screenWidth)
	cachedScreenHeight = float32(screenHeight)

	// ImGui Context Başlatma
	imgui.CreateContext()
	defer imgui.DestroyContext()

    io := imgui.CurrentIO()
    fonts := io.Fonts()
    fonts.AddFontDefault()
    fonts.Build()
	
	// Atlas'ı manuel build edip TexIsBuilt bayrağını true yapıyoruz
    pixels, width, height, _ := fonts.GetTextureDataAsRGBA32()
    _, _, _ = pixels, width, height
    fonts.SetTexID(imgui.TextureID(1))

	themeColors := []imgui.Vec4{
		{X: 1.0, Y: 0.55, Z: 0.47, W: 1.0},
		{X: 0.49, Y: 1.0, Z: 0.47, W: 1.0},
		{X: 0.48, Y: 0.47, Z: 1.0, W: 1.0},
		{X: 0.0, Y: 0.84, Z: 1.0, W: 1.0},
		{X: 0.5, Y: 0.0, Z: 0.5, W: 1.0},
		{X: 1.0, Y: 1.0, Z: 0.0, W: 1.0},
	}

	pid, err := findProcessId("cs2.exe")
	if err != nil {
		logAndSleep("Error finding process ID", err)
		return
	}

	clientDll, err := getModuleBaseAddress(pid, "client.dll")
	if err != nil {
		logAndSleep("Error getting client.dll base address", err)
		return
	}

	procHandle, err := getProcessHandle(pid)
	if err != nil {
		logAndSleep("Error getting process handle", err)
		return
	}

	offsets := getOffsets()

	running := true
	for running {
		currentHue = math.Mod(currentHue+4.0, 360)

		io.SetDisplaySize(imgui.Vec2{X: cachedScreenWidth, Y: cachedScreenHeight})

		imgui.NewFrame()

		drawList := imgui.WindowDrawList()

		if skeletonColorIndex >= len(themeColors) {
			skeletonColorIndex = 0
		}
		activeThemeCol := themeColors[skeletonColorIndex]

		entities := getEntitiesInfo(procHandle, clientDll, screenWidth, screenHeight, offsets)
		for _, entity := range entities {
			if entity.Distance < 0.8 {
				continue
			}
			if skeletonRendering {
				drawSkeletonImGui(drawList, activeThemeCol, entity.Bones)
			}
			renderEntityInfoImGui(drawList, activeThemeCol, entity.Rect, entity.Health, entity.Armor, entity.Name, entity.HeadPos, entity.Distance)
		}

		if hitmarkerRendering && time.Since(lastHitTime) < 300*time.Millisecond {
			drawHitmarkerImGui(drawList, screenWidth, screenHeight)
		}

		drawBombTimerImGui(drawList, procHandle, clientDll, screenWidth, screenHeight, offsets)
		drawInGameMenu()

		imgui.Render()

		time.Sleep(time.Millisecond * 2)
	}
}
