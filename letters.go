package main

import (
	"fmt"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/glmaljkovich/lettertracer/assets"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/audio"
	"github.com/hajimehoshi/ebiten/audio/mp3"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const (
	screenWidth       = 360
	screenHeight      = 640
	pointerOffset     = 23
	pointerSize       = 46
	sampleRate        = 44100
	animationDuration = 3200
)

// Images
var (
	brushImage       *ebiten.Image
	canvasImage      *ebiten.Image
	letterOverlay    *ebiten.Image
	pointerOverlay   *ebiten.Image
	animationOverlay *ebiten.Image
	pointerImage     *ebiten.Image
	debugImage       *ebiten.Image
	sweep            *ebiten.Image
)

// Sounds
var (
	audioContext *audio.Context
	drawPlayer   *audio.Player
	letterSounds []*audio.Player
	letterPlayer *audio.Player
)

// The rest
var (
	x, y, mx, my     int
	letterJustLoaded = true
	debugMode        bool
	animationTimer   int
	transitioning    bool
)

// Game state
// @count is used to calculate the color shift
type Game struct {
	count         int
	currentLetter int
	// letterPixels are characteristic points in a letter
	// that help us keep track of how much of it we traced over
	letterPixels []*PixelPos
}

// PixelPos position of a pixel
type PixelPos struct {
	x, y int
}

func logErrorAndExit(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func loadImages() {
	var err error
	brushImage, _, err = ebitenutil.NewImageFromFile("./assets/img/brush.png", ebiten.FilterDefault)
	logErrorAndExit(err)
	canvasImage, _, err = ebitenutil.NewImageFromFile("./assets/img/sandpaper.jpg", ebiten.FilterDefault)
	logErrorAndExit(err)
	pointerImage, _, err = ebitenutil.NewImageFromFile("./assets/img/pointer.png", ebiten.FilterDefault)
	logErrorAndExit(err)
	letterOverlay, _, err = ebitenutil.NewImageFromFile("./assets/img/a.png", ebiten.FilterDefault)
	logErrorAndExit(err)
	// 'Transparent' images
	pointerOverlay, err = ebiten.NewImage(screenWidth, screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	animationOverlay, err = ebiten.NewImage(screenWidth, screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	debugImage, err = ebiten.NewImage(screenWidth, screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	sweep, err = ebiten.NewImage(2*screenWidth, 2*screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
}

func loadAudio(src string) *audio.Player {
	file, err := ebitenutil.OpenFile(src)
	logErrorAndExit(err)

	sound, err := mp3.Decode(audioContext, file)
	logErrorAndExit(err)
	player, err := audio.NewPlayer(audioContext, sound)
	logErrorAndExit(err)
	return player
}

func loadAudios() {
	audioContext, _ = audio.NewContext(sampleRate)

	// load the pencil sound as a loop
	pencil := "./assets/audio/pencil.mp3"
	pencilSoundFile, err := ebitenutil.OpenFile(pencil)
	logErrorAndExit(err)
	pencilSound, err := mp3.Decode(audioContext, pencilSoundFile)
	logErrorAndExit(err)
	s := audio.NewInfiniteLoop(pencilSound, pencilSound.Size())
	drawPlayer, err = audio.NewPlayer(audioContext, s)
	logErrorAndExit(err)

	// load letter sounds
	for i := range assets.SoundFiles {
		letterSounds = append(letterSounds, loadAudio(assets.SoundFiles[i]))
	}
	letterPlayer = letterSounds[0]
}

func init() {
	loadImages()
	loadAudios()
}

func clearPreviousPointerState() {
	// reset the pointerOverlay
	// when the image is drawn several times this is more performant than calling Clear()
	logErrorAndExit(pointerOverlay.Dispose())
	var err error
	pointerOverlay, err = ebiten.NewImage(screenWidth, screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	// pointerOverlay.Clear()
}

func (g *Game) Updateo(screen *ebiten.Image) error {
	drawn := false
	if letterJustLoaded {
		g.letterPixels = getPixelsInLetter(letterOverlay)
		if debugMode {
			g.generateDebugImage()
		}
		letterJustLoaded = false
	}
	// Clear states
	clearPreviousPointerState()

	// Paint the brush by mouse dragging
	drawn = g.paintByMouse(screen)

	// Paint the brush by touches
	drawn = drawn || g.paintByTouches(screen)

	// Reset the pencil sound if we stopped moving
	if !drawn {
		// stop the pencil sound
		if drawPlayer.IsPlaying() {
			drawPlayer.Pause()
			drawPlayer.Rewind()
		}
	}

	// finished painting letter
	if !drawn && len(g.letterPixels) < 5 {
		g.transitionLetter()
	}

	// Show debug view with 'D'
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		debugMode = !debugMode
	}

	return nil
}

func (g *Game) Update(screen *ebiten.Image) error {
	letterPlayer.Play()
	if len(ebiten.TouchIDs()) > 0 {
		g.loadRandomLetter()
	}

	return nil
}

func (g *Game) paintByMouse(screen *ebiten.Image) bool {
	drawn := false
	mx, my := ebiten.CursorPosition()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.paint(canvasImage, mx, my)
		g.drawPointer(pointerOverlay, mx, my)
		drawn = true
	}
	return drawn
}

func (g *Game) paintByTouches(screen *ebiten.Image) bool {
	drawn := false
	for _, t := range ebiten.TouchIDs() {
		x, y := ebiten.TouchPosition(t)
		g.paint(canvasImage, x, y)
		g.drawPointer(pointerOverlay, x, y)
		drawn = true
	}
	return drawn
}

func (g *Game) transitionLetter() {
	logErrorAndExit(animationOverlay.Dispose())
	var err error
	animationOverlay, err = ebiten.NewImage(screenWidth, screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	// animationOverlay.Clear()
	if !transitioning {
		transitioning = true
		animationTimer = animationDuration
		letterPlayer.Rewind()
		// play letter pronunciation
		letterPlayer.Play()
	} else if animationTimer > animationDuration/2 {
		animationTimer -= 60
	} else if animationTimer > 0 {
		fadeOut()
		animationTimer -= 60
	} else {
		transitioning = false
		g.loadRandomLetter()
	}
}

func (g *Game) loadRandomLetter() {
	g.currentLetter = rand.Intn(len(assets.Letters))
	letter := assets.Letters[g.currentLetter]
	logErrorAndExit(letterOverlay.Dispose())
	var err error
	letterOverlay, _, err = ebitenutil.NewImageFromFile(letter, ebiten.FilterDefault)
	logErrorAndExit(err)
	// This avoids a memory leak on desktop caused by unused images being kept in memory
	logErrorAndExit(canvasImage.Dispose())
	canvasImage, _, err = ebitenutil.NewImageFromFile("./assets/img/sandpaper.jpg", ebiten.FilterDefault)
	logErrorAndExit(err)
	if letterPlayer.IsPlaying() {
		letterPlayer.Rewind()
		letterPlayer.Pause()
	}
	letterPlayer = letterSounds[g.currentLetter]
	// play letter pronunciation on load
	letterPlayer.Play()
	letterJustLoaded = true
}

// fadeOut draws an overlay with the color of the letter background
// and opacity increasing from 0 to 1
func fadeOut() {
	logErrorAndExit(sweep.Dispose())
	var err error
	sweep, err = ebiten.NewImage(2*screenWidth, 2*screenHeight, ebiten.FilterDefault)
	logErrorAndExit(err)
	// sweep.Clear()
	ccolor := letterOverlay.At(0, 0).(color.RGBA)
	sweep.Fill(ccolor)
	op := &ebiten.DrawImageOptions{}
	offset := float64(animationDuration/2-animationTimer) / float64(animationDuration/2)
	op.ColorM.Scale(1, 1, 1, offset)
	animationOverlay.DrawImage(sweep, op)
}

func (g *Game) generateDebugImage() {
	for i := range g.letterPixels {
		debugImage.Set(g.letterPixels[i].x, g.letterPixels[i].y, color.RGBA{0x40, 0xff, 0x40, 0xff})
	}
}

func getPixelsInLetter(letter *ebiten.Image) []*PixelPos {
	// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
	// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
	pixels := make([]*PixelPos, 0)
	b := letter.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if letter.At(x, y).(color.RGBA).A <= 0 {
				pixels = append(pixels, &PixelPos{x: x, y: y})
			}
		}
	}
	return getRandomPixelsInLetter(pixels)
}

func getRandomPixelsInLetter(pixels []*PixelPos) []*PixelPos {
	randomPixels := make([]*PixelPos, 0)
	for i := 0; i < 100; i++ {
		randomPixels = append(randomPixels, pixels[rand.Intn(len(pixels))])
	}
	return randomPixels
}

func insideLetter(x, y int) bool {
	// Check the actual color (alpha) value at the specified position
	return letterOverlay.At(x, y).(color.RGBA).A <= 0
}

// paint draws the brush on the given canvas image at the position (x, y).
func (g *Game) paint(canvas *ebiten.Image, x, y int) {
	op := &ebiten.DrawImageOptions{}
	// set pointer to mouse/touch position
	op.GeoM.Translate(float64(x-pointerOffset), float64(y-pointerOffset))
	// Scale the color and rotate the hue so that colors vary on each frame.
	op.ColorM.Scale(1.0, 0.75, 0.5, 1.0)
	tps := ebiten.MaxTPS()
	theta := 2.0 * math.Pi * float64(g.count%tps) / float64(tps)
	op.ColorM.RotateHue(theta)
	canvas.DrawImage(brushImage, op)
	// track progress
	if insideLetter(x, y) {
		g.removeLetterPixels(x, y)
		if !drawPlayer.IsPlaying() {
			drawPlayer.Play()
		}
		g.count++
	}
}

func (g *Game) removeLetterPixels(x, y int) {
	var newLetterPixels []*PixelPos
	for i := range g.letterPixels {
		if !isUnderPointer(x, y, g.letterPixels[i]) {
			newLetterPixels = append(newLetterPixels, g.letterPixels[i])
		} else {
			debugImage.Set(x, y, color.RGBA{0x40, 0x40, 0xff, 0xff})
		}
	}
	g.letterPixels = newLetterPixels
}

// Check if a pixel is under the entire shape of the pointer image
func isUnderPointer(px, py int, pixel *PixelPos) bool {
	inXBounds := ((px + pointerSize/2) >= pixel.x) && ((px - pointerSize/2) <= pixel.x)
	inYBounds := ((py + pointerSize/2) >= pixel.y) && ((py - pointerSize/2) <= pixel.y)
	return inXBounds && inYBounds
}

func (g *Game) drawPointer(pointerOverlay *ebiten.Image, x, y int) {
	op := &ebiten.DrawImageOptions{}

	op.GeoM.Translate(float64(x-pointerOffset), float64(y-pointerOffset))
	pointerOverlay.DrawImage(pointerImage, op)
}

func (g *Game) renderDebugScreen(screen *ebiten.Image) {
	screen.DrawImage(debugImage, nil)
	mx, my := ebiten.CursorPosition()
	msg := fmt.Sprintf("(%d, %d), %d left", mx, my, len(g.letterPixels))
	for _, t := range ebiten.TouchIDs() {
		x, y := ebiten.TouchPosition(t)
		msg += fmt.Sprintf("\n(%d, %d) touch %d", x, y, t)
	}
	ebitenutil.DebugPrint(screen, msg)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.DrawImage(canvasImage, nil)
	screen.DrawImage(letterOverlay, nil)
	screen.DrawImage(animationOverlay, nil)
	screen.DrawImage(pointerOverlay, nil)
	if debugMode {
		g.renderDebugScreen(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	// Initialize rand otherwise we'll always get the same sequence
	rand.Seed(time.Now().UnixNano())
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("letters")
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}
