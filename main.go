package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unsafe"

	"github.com/lxn/win"
	"github.com/ttacon/chalk"
	"golang.org/x/sys/windows"
)

//go:embed offsets.json
var embeddedOffsets []byte

//go:embed hit.wav
var embeddedHitSound []byte

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
	DwViewMatrix         uintptr `json:"dwViewMatrix"`
	DwLocalPlayerPawn    uintptr `json:"dwLocalPlayerPawn"`
	DwEntityList         uintptr `json:"dwEntityList"`
	M_hPlayerPawn        uintptr `json:"m_hPlayerPawn"`
	M_iHealth            uintptr `json:"m_iHealth"`
	M_lifeState          uintptr `json:"m_lifeState"`
	M_iTeamNum           uintptr `json:"m_iTeamNum"`
	M_vOldOrigin         uintptr `json:"m_vOldOrigin"`
	M_pGameSceneNode     uintptr `json:"m_pGameSceneNode"`
	M_modelState         uintptr `json:"m_modelState"`
	M_boneArray          uintptr `json:"m_boneArray"`
	M_nodeToWorld        uintptr `json:"m_nodeToWorld"`
	M_sSanitizedPlayerName uintptr `json:"m_sSanitizedPlayerName"`
	M_ArmorValue         uintptr `json:"m_ArmorValue"`
	M_pBulletServices    uintptr `json:"m_pBulletServices"`    // YENİ: Bullet Services
	M_totalHitsOnServer  uintptr `json:"m_totalHitsOnServer"` // YENİ: Total Hits
}

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	gdi32                      = windows.NewLazySystemDLL("gdi32.dll")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	winmm                      = windows.NewLazySystemDLL("winmm.dll") // YENİ: Ses için DLL
	getSystemMetrics           = user32.NewProc("GetSystemMetrics")
	setLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	setWindowDisplayAffinity   = user32.NewProc("SetWindowDisplayAffinity")
	showCursor                 = user32.NewProc("ShowCursor")
	setTextAlign               = gdi32.NewProc("SetTextAlign")
	createFont                 = gdi32.NewProc("CreateFontW")
	createCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	createSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	createPen                  = gdi32.NewProc("CreatePen")
	GetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	beepProc                   = kernel32.NewProc("Beep")
	playSoundProc              = winmm.NewProc("PlaySoundW") // YENİ: Ses oynatma fonksiyonu
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
	hitmarkerRendering  bool    = true // YENİ: Hitmarker
	hitSoundRendering   bool    = true // YENİ: Hit Sound
	frameDelay          uint32  = 15
	currentHue          float64 = 0

	// Hitmarker takibi için değişkenler
	lastHits    int32 = -1
	lastHitTime time.Time
)

const asciiArt = `
 _____    ___________ _________ _______    _______________    ____.___ 
/     \   \_     __//   _____/ \      \   \_   _____/\   \  /   /|   |
\     \ /  \   | __)\_____  \  /   |   \   |    __)_  \   Y   / |   |
 \     Y    /   |    \/       \/    |    \  |        \  \     /  |   |
  \___|__  /_______  /_______  /\____|__  /_______  /   \___/   |___|
         \/        \/        \/         \/        \/                 `

var (
	bgBrush            uintptr
	redPen             uintptr
	greenPen           uintptr
	bluePen            uintptr
	bonePen            uintptr
	blueBonePen        uintptr
	greenBonePen       uintptr
	redBonePen         uintptr
	outlinePen         uintptr
	armorPen           uintptr
	hitmarkerPen       uintptr // YENİ: Hitmarker kalemi
	skeletonColor      uintptr
	skeletonColorIndex int
)

func init() {
	runtime.LockOSThread()
	rand.Seed(time.Now().UnixNano())
	cachedHitPath = extractEmbeddedHitSound()
}

func extractEmbeddedHitSound() string {
	tmpPath := filepath.Join(os.TempDir(), "hit.wav")
	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		_ = os.WriteFile(tmpPath, embeddedHitSound, 0644)
	}
	return tmpPath
}

func hsvToRGB(h, s, v float64) (byte, byte, byte) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := v - c
	var r1, g1, b1 float64
	switch {
	case 0 <= h && h < 60:
		r1, g1, b1 = c, x, 0
	case 60 <= h && h < 120:
		r1, g1, b1 = x, c, 0
	case 120 <= h && h < 180:
		r1, g1, b1 = 0, c, x
	case 180 <= h && h < 240:
		r1, g1, b1 = 0, x, c
	case 240 <= h && h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	return byte((r1 + m) * 255), byte((g1 + m) * 255), byte((b1 + m) * 255)
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

// YENİ: Hit Sound oynatma fonksiyonu
func playHitSound() {
	soundPath, _ := windows.UTF16PtrFromString("hit.wav")
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
	width, _, _ := getSystemMetrics.Call(0)
	height, _, _ := getSystemMetrics.Call(1)
	widthFloat := float32(width)
	heightFloat := float32(height)
	x := widthFloat / 2
	y := heightFloat / 2
	x += 0.5*screenX*widthFloat + 0.5
	y -= 0.5*screenY*heightFloat + 0.5
	return x, y
}

func getOffsets() Offset {
	var offsets Offset
	err := json.Unmarshal(embeddedOffsets, &offsets)
	if err != nil {
		fmt.Println("Error decoding embedded offsets JSON:", err)
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
	if err != nil {
		return entities
	}

	// YENİ: Local Player'ın mermi isabet sayısını (hits) okuma
	if hitmarkerRendering {
		var bulletServices uintptr
		var totalHits int32
		err = read(procHandle, localPlayerP+offsets.M_pBulletServices, &bulletServices)
		if err == nil && bulletServices != 0 {
			err = read(procHandle, bulletServices+offsets.M_totalHitsOnServer, &totalHits)
			if err == nil {
				if lastHits == -1 {
					lastHits = totalHits // İlk defa okunuyorsa eşitle
				} else if totalHits > lastHits {
					lastHitTime = time.Now()
					if hitSoundRendering {
						go playHitSound()
					}
					lastHits = totalHits
				} else if totalHits < lastHits {
					lastHits = totalHits // Map değiştiğinde veya öldüğünde sıfırlanma durumu
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

		for boneName, boneIndex := range bones {
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

func drawSkeleton(hdc win.HDC, pen uintptr, bones map[string]Vector2) {
	win.SelectObject(hdc, win.HGDIOBJ(pen))
	win.MoveToEx(hdc, int(bones["head"].X), int(bones["head"].Y), nil)
	win.LineTo(hdc, int32(bones["neck_0"].X), int32(bones["neck_0"].Y))
	win.LineTo(hdc, int32(bones["spine_1"].X), int32(bones["spine_1"].Y))
	win.LineTo(hdc, int32(bones["spine_2"].X), int32(bones["spine_2"].Y))
	win.LineTo(hdc, int32(bones["pelvis"].X), int32(bones["pelvis"].Y))
	win.LineTo(hdc, int32(bones["leg_upper_L"].X), int32(bones["leg_upper_L"].Y))
	win.LineTo(hdc, int32(bones["leg_lower_L"].X), int32(bones["leg_lower_L"].Y))
	win.LineTo(hdc, int32(bones["ankle_L"].X), int32(bones["ankle_L"].Y))
	win.MoveToEx(hdc, int(bones["pelvis"].X), int(bones["pelvis"].Y), nil)
	win.LineTo(hdc, int32(bones["leg_upper_R"].X), int32(bones["leg_upper_R"].Y))
	win.LineTo(hdc, int32(bones["leg_lower_R"].X), int32(bones["leg_lower_R"].Y))
	win.LineTo(hdc, int32(bones["ankle_R"].X), int32(bones["ankle_R"].Y))
	win.MoveToEx(hdc, int(bones["spine_1"].X), int(bones["spine_1"].Y), nil)
	win.LineTo(hdc, int32(bones["arm_upper_L"].X), int32(bones["arm_upper_L"].Y))
	win.LineTo(hdc, int32(bones["arm_lower_L"].X), int32(bones["arm_lower_L"].Y))
	win.LineTo(hdc, int32(bones["hand_L"].X), int32(bones["hand_L"].Y))
	win.MoveToEx(hdc, int(bones["spine_1"].X), int(bones["spine_1"].Y), nil)
	win.LineTo(hdc, int32(bones["arm_upper_R"].X), int32(bones["arm_upper_R"].Y))
	win.LineTo(hdc, int32(bones["arm_lower_R"].X), int32(bones["arm_lower_R"].Y))
	win.LineTo(hdc, int32(bones["hand_R"].X), int32(bones["hand_R"].Y))
}

// YENİ: Hitmarker'ı ekranın tam ortasına çizme fonksiyonu
func drawHitmarker(hdc win.HDC, pen uintptr, screenWidth, screenHeight uintptr) {
	centerX := int32(screenWidth / 2)
	centerY := int32(screenHeight / 2)
	size := int32(8)
	gap := int32(4)

	win.SelectObject(hdc, win.HGDIOBJ(pen))

	// Sol Üst'ten Sağ Alt'a
	win.MoveToEx(hdc, int(centerX-size), int(centerY-size), nil)
	win.LineTo(hdc, int32(centerX-gap), int32(centerY-gap))
	win.MoveToEx(hdc, int(centerX+gap), int(centerY+gap), nil)
	win.LineTo(hdc, int32(centerX+size), int32(centerY+size))

	// Sağ Üst'ten Sol Alt'a
	win.MoveToEx(hdc, int(centerX+size), int(centerY-size), nil)
	win.LineTo(hdc, int32(centerX+gap), int32(centerY-gap))
	win.MoveToEx(hdc, int(centerX-gap), int(centerY+gap), nil)
	win.LineTo(hdc, int32(centerX-size), int32(centerY+size))
}

func createDynamicHealthPen(hp int32) uintptr {
	if hp < 0 {
		hp = 0
	} else if hp > 100 {
		hp = 100
	}

	r := byte((100 - hp) * 255 / 100)
	g := byte(hp * 255 / 100)

	color := uintptr(r) | (uintptr(g) << 8)
	pen, _, _ := createPen.Call(win.PS_SOLID, 2, color)
	return pen
}

func renderEntityInfo(hdc win.HDC, tPen uintptr, gPen uintptr, oPen uintptr, hPen uintptr, aPen uintptr, rect Rectangle, hp int32, armor int32, name string, headPos Vector3, distance float32) {
	if boxRendering {
		win.SelectObject(hdc, win.HGDIOBJ(tPen))
		win.MoveToEx(hdc, int(rect.Left), int(rect.Top), nil)
		win.LineTo(hdc, int32(rect.Right), int32(rect.Top))
		win.LineTo(hdc, int32(rect.Right), int32(rect.Bottom))
		win.LineTo(hdc, int32(rect.Left), int32(rect.Bottom))
		win.LineTo(hdc, int32(rect.Left), int32(rect.Top))

		win.SelectObject(hdc, win.HGDIOBJ(oPen))
		win.MoveToEx(hdc, int(rect.Left)-1, int(rect.Top)-1, nil)
		win.LineTo(hdc, int32(rect.Right)-1, int32(rect.Top)+1)
		win.LineTo(hdc, int32(rect.Right)+1, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Left)+1, int32(rect.Bottom)-1)
		win.LineTo(hdc, int32(rect.Left)-1, int32(rect.Top)-1)
		win.MoveToEx(hdc, int(rect.Left)+1, int(rect.Top)+1, nil)
		win.LineTo(hdc, int32(rect.Right)+1, int32(rect.Top)-1)
		win.LineTo(hdc, int32(rect.Right)-1, int32(rect.Bottom)-1)
		win.LineTo(hdc, int32(rect.Left)-1, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Left)+1, int32(rect.Top)+1)
	}

	if headCircle {
		radius := int32((int32(headPos.Z) - int32(headPos.Y)) / 2)
		win.SelectObject(hdc, win.HGDIOBJ(oPen))
		win.Ellipse(hdc, int32(headPos.X)-radius-1, int32(headPos.Y)-1, int32(headPos.X)+radius+1, int32(headPos.Z)+1)
		win.SelectObject(hdc, win.HGDIOBJ(hPen))
		win.Ellipse(hdc, int32(headPos.X)-radius, int32(headPos.Y), int32(headPos.X)+radius, int32(headPos.Z))
		win.SelectObject(hdc, win.HGDIOBJ(oPen))
		win.Ellipse(hdc, int32(headPos.X)-radius+1, int32(headPos.Y)+1, int32(headPos.X)+radius-1, int32(headPos.Z)-1)
	}

	if healthBarRendering {
		dynamicHpPen := createDynamicHealthPen(hp)

		win.SelectObject(hdc, win.HGDIOBJ(dynamicHpPen))
		win.MoveToEx(hdc, int(rect.Left)-4, int(rect.Bottom)+1-int(float64(int(rect.Bottom)+1-int(rect.Top))*float64(hp)/100.0), nil)
		win.LineTo(hdc, int32(rect.Left)-4, int32(rect.Bottom)+1)

		win.DeleteObject(win.HGDIOBJ(dynamicHpPen))

		win.SelectObject(hdc, win.HGDIOBJ(oPen))
		win.MoveToEx(hdc, int(rect.Left)-5, int(rect.Top)-1, nil)
		win.LineTo(hdc, int32(rect.Left)-5, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Left)-3, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Left)-3, int32(rect.Top)-1)
		win.LineTo(hdc, int32(rect.Left)-5, int32(rect.Top)-1)
	}

	if armorBarRendering && armor > 0 {
		if armor > 100 {
			armor = 100
		}
		
		yPos := int(rect.Bottom) + 1 - int(float64(int(rect.Bottom)+1-int(rect.Top))*float64(armor)/100.0)

		win.SelectObject(hdc, win.HGDIOBJ(aPen))
		win.MoveToEx(hdc, int(rect.Right)+4, yPos, nil)
		win.LineTo(hdc, int32(rect.Right)+4, int32(rect.Bottom)+1)

		win.SelectObject(hdc, win.HGDIOBJ(oPen))
		win.MoveToEx(hdc, int(rect.Right)+3, int(rect.Top)-1, nil)
		win.LineTo(hdc, int32(rect.Right)+3, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Right)+5, int32(rect.Bottom)+1)
		win.LineTo(hdc, int32(rect.Right)+5, int32(rect.Top)-1)
		win.LineTo(hdc, int32(rect.Right)+3, int32(rect.Top)-1)

		armorText, _ := windows.UTF16PtrFromString(fmt.Sprintf("%d", armor))
		win.SetTextColor(hdc, win.RGB(byte(0), byte(204), byte(255))) 
		setTextAlign.Call(uintptr(hdc), 0x00000000) 
		win.TextOut(hdc, int32(rect.Right)+8, int32(yPos)-4, armorText, int32(len(fmt.Sprintf("%d", armor))))
	}

	if healthTextRendering {
		text, _ := windows.UTF16PtrFromString(fmt.Sprintf("%d", hp))
		win.SetTextColor(hdc, win.RGB(byte(0), byte(255), byte(50)))
		setTextAlign.Call(uintptr(hdc), 0x00000002) 
		if healthBarRendering {
			win.TextOut(hdc, int32(rect.Left)-8, int32(int(rect.Bottom)+1-int(float64(int(rect.Bottom)+1-int(rect.Top))*float64(hp)/100.0)), text, int32(len(fmt.Sprintf("%d", hp))))
		} else {
			win.TextOut(hdc, int32(rect.Left)-4, int32(rect.Top), text, int32(len(fmt.Sprintf("%d", hp))))
		}
	}

	if nameRendering {
		text, _ := windows.UTF16PtrFromString(name)
		win.SetTextColor(hdc, win.RGB(byte(255), byte(255), byte(255)))
		setTextAlign.Call(uintptr(hdc), 0x00000006)
		win.TextOut(hdc, int32(rect.Left)+int32((int32(rect.Right)-int32(rect.Left))/2), int32(rect.Top)-14, text, int32(len(name)))
	}

	if distanceRendering {
		distStr := fmt.Sprintf("%.0fm", distance)
		text, _ := windows.UTF16PtrFromString(distStr)
		win.SetTextColor(hdc, win.RGB(byte(255), byte(255), byte(150)))
		setTextAlign.Call(uintptr(hdc), 0x00000006)
		win.TextOut(hdc, int32(rect.Left)+int32((int32(rect.Right)-int32(rect.Left))/2), int32(rect.Bottom)+2, text, int32(len(distStr)))
	}
}

func windowProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_TIMER:
		return 0
	case win.WM_DESTROY:
		win.PostQuitMessage(0)
		return 0
	default:
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
}

func initWindow(screenWidth uintptr, screenHeight uintptr) win.HWND {
	randClassName := generateRandomString(12)
	randWindowTitle := generateRandomString(16)

	className, err := windows.UTF16PtrFromString(randClassName)
	if err != nil {
		logAndSleep("Error creating window class name", err)
		return 0
	}
	windowTitle, err := windows.UTF16PtrFromString(randWindowTitle)
	if err != nil {
		logAndSleep("Error creating window title", err)
		return 0
	}

	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(windowProc),
		CbWndExtra:    0,
		HInstance:     win.GetModuleHandle(nil),
		HIcon:         win.LoadIcon(0, (*uint16)(unsafe.Pointer(uintptr(win.IDI_APPLICATION)))),
		HCursor:       win.LoadCursor(0, (*uint16)(unsafe.Pointer(uintptr(win.IDC_ARROW)))),
		HbrBackground: win.COLOR_WINDOW,
		LpszMenuName:  nil,
		LpszClassName: className,
		HIconSm:       win.LoadIcon(0, (*uint16)(unsafe.Pointer(uintptr(win.IDI_APPLICATION)))),
	}

	if atom := win.RegisterClassEx(&wc); atom == 0 {
		logAndSleep("Error registering window class", fmt.Errorf("%v", win.GetLastError()))
		return 0
	}

	hInstance := win.GetModuleHandle(nil)
	hwnd := win.CreateWindowEx(
		win.WS_EX_TOPMOST|win.WS_EX_NOACTIVATE|win.WS_EX_LAYERED,
		className,
		windowTitle,
		win.WS_POPUP,
		0,
		0,
		int32(screenWidth),
		int32(screenHeight),
		0,
		0,
		hInstance,
		nil,
	)
	if hwnd == 0 {
		logAndSleep("Error creating window", fmt.Errorf("%v", win.GetLastError()))
		return 0
	}

	result, _, _ := setLayeredWindowAttributes.Call(uintptr(hwnd), 0x000000, 0, 0x00000001)
	if result == 0 {
		logAndSleep("Error setting layered window attributes", fmt.Errorf("%v", win.GetLastError()))
	}
	style := win.GetWindowLongPtr(hwnd, win.GWL_EXSTYLE)
	style |= win.WS_EX_TRANSPARENT
	win.SetWindowLongPtr(hwnd, win.GWL_EXSTYLE, style)

	const WDA_EXCLUDEFROMCAPTURE = 0x00000011
	setWindowDisplayAffinity.Call(uintptr(hwnd), uintptr(WDA_EXCLUDEFROMCAPTURE))

	showCursor.Call(0)
	win.ShowWindow(hwnd, win.SW_SHOWDEFAULT)
	return hwnd
}

func printStatus(label string, status bool) {
	if status {
		fmt.Printf("  %-25s %s\n", label, chalk.Green.Color("[ AÇIK ]"))
	} else {
		fmt.Printf("  %-25s %s\n", label, chalk.Red.Color("[ KAPALI ]"))
	}
}

func renderUI() {
	fmt.Print("\033[H")

	lines := strings.Split(asciiArt, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lineHue := math.Mod(currentHue+float64(i*25), 360)
		r, g, b := hsvToRGB(lineHue, 1.0, 1.0)
		fmt.Printf("\x1b[38;2;%d;%d;%dm%s\x1b[0m\n", r, g, b, line)
	}

	fmt.Println(chalk.Yellow.Color("        by bxiefr - v1.8.3\n"))
	fmt.Println(chalk.Cyan.Color("=========================================="))
	fmt.Println(chalk.White.Color("          KISAYOL TUŞLARI (HOTKEYS)"))
	fmt.Println(chalk.Cyan.Color("=========================================="))

	printStatus("[F1] Box ESP", boxRendering)
	printStatus("[F2] Skeleton ESP", skeletonRendering)
	printStatus("[F3] Head Circle", headCircle)
	printStatus("[F4] Team Check", teamCheck)
	printStatus("[F5] Health Bar", healthBarRendering)
	printStatus("[F6] Player Name", nameRendering)
	printStatus("[F7] Distance Display", distanceRendering)
	printStatus("[F9] Armor Bar", armorBarRendering)
	printStatus("[F10] Hitmarker & Sound", hitmarkerRendering) // YENİ

	colorNames := []string{"MAVİ", "YEŞİL", "KIRMIZI"}
	currentColor := colorNames[skeletonColorIndex]
	fmt.Printf("  %-25s %s\n", "[F8] Skeleton Color", chalk.Yellow.Color("["+currentColor+"]"))

	fmt.Println(chalk.Cyan.Color("------------------------------------------"))
	fmt.Println(chalk.Red.Color("  [END] Hileyi Kapat / Close Cheat"))
	fmt.Println(chalk.Cyan.Color("=========================================="))
}

func startRGBUI() {
	for {
		currentHue = math.Mod(currentHue+3.0, 360)
		renderUI()
		time.Sleep(40 * time.Millisecond)
	}
}

func isKeyPressed(vKey int) bool {
	ret, _, _ := GetAsyncKeyState.Call(uintptr(vKey))
	return (ret & 0x0001) != 0
}

func listenHotkeys() {
	for {
		if isKeyPressed(0x77) { // F8
			skeletonColorIndex = (skeletonColorIndex + 1) % 3
			if skeletonColorIndex == 0 {
				skeletonColor = blueBonePen
			} else if skeletonColorIndex == 1 {
				skeletonColor = greenBonePen
			} else if skeletonColorIndex == 2 {
				skeletonColor = redBonePen
			}
			playBeep(true)
		}
		if isKeyPressed(0x70) { // F1
			boxRendering = !boxRendering
			playBeep(boxRendering)
		}
		if isKeyPressed(0x71) { // F2
			skeletonRendering = !skeletonRendering
			playBeep(skeletonRendering)
		}
		if isKeyPressed(0x72) { // F3
			headCircle = !headCircle
			playBeep(headCircle)
		}
		if isKeyPressed(0x73) { // F4
			teamCheck = !teamCheck
			playBeep(teamCheck)
		}
		if isKeyPressed(0x74) { // F5
			healthBarRendering = !healthBarRendering
			playBeep(healthBarRendering)
		}
		if isKeyPressed(0x75) { // F6
			nameRendering = !nameRendering
			playBeep(nameRendering)
		}
		if isKeyPressed(0x76) { // F7
			distanceRendering = !distanceRendering
			playBeep(distanceRendering)
		}
		if isKeyPressed(0x78) { // F9
			armorBarRendering = !armorBarRendering
			playBeep(armorBarRendering)
		}
		if isKeyPressed(0x79) { // YENİ: F10 tuşu Hitmarker & Sound
			hitmarkerRendering = !hitmarkerRendering
			hitSoundRendering = hitmarkerRendering
			playBeep(hitmarkerRendering)
		}
		if isKeyPressed(0x23) { // END
			playBeep(false)
			os.Exit(0)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func main() {
	fmt.Print("\033[H\033[2J") 
	go startRGBUI()
	go listenHotkeys()

	screenWidth, _, _ := getSystemMetrics.Call(0)
	screenHeight, _, _ := getSystemMetrics.Call(1)

	hwnd := initWindow(screenWidth, screenHeight)
	if hwnd == 0 {
		logAndSleep("Error creating window", fmt.Errorf("%v", win.GetLastError()))
		return
	}
	defer win.DestroyWindow(hwnd)

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

	hdc := win.GetDC(hwnd)
	if hdc == 0 {
		logAndSleep("Error getting device context", fmt.Errorf("%v", win.GetLastError()))
		return
	}

	var errCreate error
	bgBrush, _, errCreate = createSolidBrush.Call(uintptr(0x000000))
	if bgBrush == 0 {
		logAndSleep("Error creating brush", errCreate)
		return
	}
	defer win.DeleteObject(win.HGDIOBJ(bgBrush))

	redPen, _, _ = createPen.Call(win.PS_SOLID, 1, 0x7a78ff)
	if redPen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(redPen))

	greenPen, _, _ = createPen.Call(win.PS_SOLID, 1, 0x7dff78)
	if greenPen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(greenPen))

	bluePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0xff8e78)
	if bluePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(bluePen))

	bonePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0xffffff)
	if bonePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(bonePen))

	blueBonePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0xff8e78)
	if blueBonePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(blueBonePen))

	greenBonePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0x7dff78)
	if greenBonePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(greenBonePen))

	redBonePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0x7a78ff)
	if redBonePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(redBonePen))

	outlinePen, _, _ = createPen.Call(win.PS_SOLID, 1, 0x000000)
	if outlinePen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(outlinePen))

	armorPen, _, _ = createPen.Call(win.PS_SOLID, 2, 0xffcc00)
	if armorPen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(armorPen))

	// YENİ: Hitmarker Kalemi
	hitmarkerPen, _, _ = createPen.Call(win.PS_SOLID, 2, 0xffffff)
	if hitmarkerPen == 0 { return }
	defer win.DeleteObject(win.HGDIOBJ(hitmarkerPen))

	skeletonColor = blueBonePen

	font, _, _ := createFont.Call(12, 0, 0, 0, win.FW_HEAVY, 0, 0, 0, win.DEFAULT_CHARSET, win.OUT_DEFAULT_PRECIS, win.CLIP_DEFAULT_PRECIS, win.DEFAULT_QUALITY, win.DEFAULT_PITCH|win.FF_DONTCARE, 0)

	offsets := getOffsets()

	var msg win.MSG

	for win.GetMessage(&msg, 0, 0, 0) > 0 {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)

		win.SetTimer(hwnd, 1, frameDelay, 0)

		memhdc, _, _ := createCompatibleDC.Call(uintptr(hdc))
		memBitmap := win.CreateCompatibleBitmap(hdc, int32(screenWidth), int32(screenHeight))
		win.SelectObject(win.HDC(memhdc), win.HGDIOBJ(memBitmap))
		win.SelectObject(win.HDC(memhdc), win.HGDIOBJ(bgBrush))
		win.SetBkMode(win.HDC(memhdc), win.TRANSPARENT)
		win.SelectObject(win.HDC(memhdc), win.HGDIOBJ(font))

		entities := getEntitiesInfo(procHandle, clientDll, screenWidth, screenHeight, offsets)
		for _, entity := range entities {
			if entity.Distance < 0.8 {
				continue
			}
			if skeletonRendering {
				drawSkeleton(win.HDC(memhdc), skeletonColor, entity.Bones)
			}
			if entity.Team == 2 {
				renderEntityInfo(win.HDC(memhdc), redPen, greenPen, outlinePen, bonePen, armorPen, entity.Rect, entity.Health, entity.Armor, entity.Name, entity.HeadPos, entity.Distance)
			} else {
				renderEntityInfo(win.HDC(memhdc), bluePen, greenPen, outlinePen, bonePen, armorPen, entity.Rect, entity.Health, entity.Armor, entity.Name, entity.HeadPos, entity.Distance)
			}
		}

		// YENİ: Vuruş yapıldıysa son 300 ms boyunca ekranın ortasına Hitmarker'ı çiz
		if hitmarkerRendering && time.Since(lastHitTime) < 300*time.Millisecond {
			drawHitmarker(win.HDC(memhdc), hitmarkerPen, screenWidth, screenHeight)
		}

		win.BitBlt(hdc, 0, 0, int32(screenWidth), int32(screenHeight), win.HDC(memhdc), 0, 0, win.SRCCOPY)

		win.DeleteObject(win.HGDIOBJ(memBitmap))
		win.DeleteDC(win.HDC(memhdc))
	}
}
