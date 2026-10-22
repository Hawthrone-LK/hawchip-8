package main

import (
    "bufio"
    "fmt"
    "log"
    "math/rand"
    "os"
    "strings"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/audio"
)

// Builds the audio system
const sampleRate = 11025

type SquareWaveStream struct {
    sampleRate uint32
    t          uint32
    freq       uint32
}

// creates the Square Wave Generator
func (s *SquareWaveStream) Read(buf []byte) (int, error) {
    length := len(buf)
    period := s.sampleRate / s.freq
    if period == 0 {
        period = 1
    }
    halfPeriod := period / 2

    for i := 0; i < length; i += 4 {
        s.t++
        var sample int16
        if (s.t/halfPeriod)%2 == 0 {
            sample = 3000
        } else {
            sample = -3000
        }

        buf[i] = byte(sample)
        buf[i+1] = byte(sample >> 8)
        buf[i+2] = byte(sample)
        buf[i+3] = byte(sample >> 8)
    }
    return length, nil
}

// stupid dummy function
func (s *SquareWaveStream) Seek(offset int64, whence int) (int64, error) {
    s.t = 0
    return 0, nil
} // have I mentioned that I hate abstraction layers?


// the heart of the CHIP, declares the RAM, I/O and CPU registers
type Chip8 struct {
    ram         [4096]byte
    registers   [16]byte
    index       uint16
    pc          uint16
    stack       [16]uint16
    sp          byte
    delayTimer  byte
    soundTimer  byte
    display     [64 * 32]byte
    keys        [16]bool

    audioPlayer *audio.Player
    debugMode   bool
    debugFile   *os.File
}

// Boot up the new virtual machine
func NewChip8(debug bool) *Chip8 {
    c := &Chip8{
        debugMode: debug,
    }
    c.pc = 0x200

    // Standartized default Font Set of CHIP-8
    fontSet := [80]byte{
        0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
        0x20, 0x60, 0x20, 0x20, 0x70, // 1
        0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
        0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
        0x90, 0x90, 0xF0, 0x10, 0x10, // 4
        0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
        0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
        0xF0, 0x10, 0x20, 0x40, 0x40, // 7
        0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
        0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
        0xF0, 0x90, 0xF0, 0x90, 0x90, // A
        0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
        0xF0, 0x80, 0x80, 0x80, 0xF0, // C
        0xE0, 0x90, 0x90, 0x90, 0xE0, // D
        0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
        0xF0, 0x80, 0xF0, 0x80, 0x80, // F
    }
    copy(c.ram[0x050:], fontSet[:])

    // A Dev Tool created for Test and Analysis
    // If enable, generates a .TXT file with every bytecode executed in the session
    if c.debugMode {
        file, err := os.Create("./disk_system/debug_log.txt")
        if err == nil {
            c.debugFile = file
            c.logDebug("[DEBUGGER] HawCHIP-8 Debug Mode Activated. Logging instructions...\n")
        }
    }

    // Start audio context
    audioCtx := audio.NewContext(sampleRate)
    wave := &SquareWaveStream{
        sampleRate: sampleRate,
        freq:       440 / 2,
    }
    infiniteStream := audio.NewInfiniteLoop(wave, 10<<20)
    player, err := audioCtx.NewPlayer(infiniteStream)
    if err == nil {
        c.audioPlayer = player
    }
    
    return c
}

// The main update function
// Creates a 60Hz refresh cycle, permiting the Fetch-Decode-Execute logic cycle
func (c *Chip8) logDebug(format string, a ...interface{}) {
    msg := fmt.Sprintf(format, a...)
    if c.debugMode && c.debugFile != nil {
        c.debugFile.WriteString(msg)
    }
}

// Set up the Delay and Update Timer
func (c *Chip8) Update() error {
    if c.delayTimer > 0 {
        c.delayTimer--
    }
    
    if c.soundTimer > 0 {
        c.soundTimer--
        if c.audioPlayer != nil && !c.audioPlayer.IsPlaying() {
            c.audioPlayer.Play()
        }
    } else {
        if c.audioPlayer != nil && c.audioPlayer.IsPlaying() {
            c.audioPlayer.Pause()
        }
    }

    // Allows the Input Handling
    // You can remap by just changing the ".Key{x}"
    c.keys[0x1] = ebiten.IsKeyPressed(ebiten.Key1)
    c.keys[0x2] = ebiten.IsKeyPressed(ebiten.Key2)
    c.keys[0x3] = ebiten.IsKeyPressed(ebiten.Key3)
    c.keys[0xC] = ebiten.IsKeyPressed(ebiten.Key4)

    c.keys[0x4] = ebiten.IsKeyPressed(ebiten.KeyQ)
    c.keys[0x5] = ebiten.IsKeyPressed(ebiten.KeyW)
    c.keys[0x6] = ebiten.IsKeyPressed(ebiten.KeyE)
    c.keys[0xD] = ebiten.IsKeyPressed(ebiten.KeyR)

    c.keys[0x7] = ebiten.IsKeyPressed(ebiten.KeyA)
    c.keys[0x8] = ebiten.IsKeyPressed(ebiten.KeyS)
    c.keys[0x9] = ebiten.IsKeyPressed(ebiten.KeyD)
    c.keys[0xE] = ebiten.IsKeyPressed(ebiten.KeyF)

    c.keys[0xA] = ebiten.IsKeyPressed(ebiten.KeyZ)
    c.keys[0x0] = ebiten.IsKeyPressed(ebiten.KeyX)
    c.keys[0xB] = ebiten.IsKeyPressed(ebiten.KeyC)
    c.keys[0xF] = ebiten.IsKeyPressed(ebiten.KeyV)

    // Initiate the Fetch cycle
    // Search opcodes from RAM starting at 0x200 (512) address
    // Merge 2 bytes (HighByte & LowByte) at one single opcode
    for i := 0; i < 10; i++ {
        if c.pc >= 4095 {
            return nil
        }
    
        highByte := c.ram[c.pc]
        lowByte := c.ram[c.pc+1]
        opcode := (uint16(highByte) << 8) | uint16(lowByte)

        if opcode == 0x0000 {
            c.pc += 2
            continue
        }

        nibble1 := (opcode & 0xF000) >> 12
        x := (opcode & 0x0F00) >> 8
        y := (opcode & 0x00F0) >> 4
        n := (opcode & 0x000F)
        nn := byte(opcode & 0x00FF)
        nnn := opcode & 0x0FFF

        pcJumped := false

        // The big tree of the opcode Matching Pattern... 35 opcodes
        switch nibble1 {
        case 0x0:
            switch nn {
            case 0xE0:
                c.display = [64 * 32]byte{}
                c.logDebug("[EXECUTE] 00E0: Clear Screen\n")
            case 0xEE:
                c.sp--
                c.pc = c.stack[c.sp]
                pcJumped = true
                c.logDebug("[EXECUTE] 00EE: Return to PC 0x%04X\n", c.pc)
            }

        case 0x1:
            c.pc = nnn
            pcJumped = true
            c.logDebug("[EXECUTE] 1NNN: Teleporting PC to 0x%04X\n", nnn)

        case 0x2:
            c.stack[c.sp] = c.pc + 2
            c.sp++
            c.pc = nnn
            pcJumped = true
            c.logDebug("[EXECUTE] 2NNN: Calling subroutine at 0x%04X\n", nnn)

        case 0x3:
            if c.registers[x] == nn {
                c.pc += 2
            }
            c.logDebug("[EXECUTE] 3XNN: Condition check on V%X\n", x)

        case 0x4:
            if c.registers[x] != nn {
                c.pc += 2
            }
            c.logDebug("[EXECUTE] 4XNN: Condition check on V%X\n", x)

        case 0x5:
            if c.registers[x] == c.registers[y] {
                c.pc += 2
            }
            c.logDebug("[EXECUTE] 5XY0: Skip if V%X == V%X\n", x, y)

        case 0x6:
            c.registers[x] = nn
            c.logDebug("[EXECUTE] 6XNN: Stored value 0x%02X into Register V%X\n", nn, x)

        case 0x7:
            c.registers[x] += nn
            c.logDebug("[EXECUTE] 7XNN: Added value 0x%02X to Register V%X\n", nn, x)
            
        // Arithmetic operators
        case 0x8:
            switch n {
            case 0x0:
                c.registers[x] = c.registers[y]
                c.logDebug("[EXECUTE] 8XY0: V%X = V%X\n", x, y)
            case 0x1:
                c.registers[x] |= c.registers[y]
                c.logDebug("[EXECUTE] 8XY1: V%X |= V%X\n", x, y)
            case 0x2:
                c.registers[x] &= c.registers[y]
                c.logDebug("[EXECUTE] 8XY2: V%X &= V%X\n", x, y)
            case 0x3:
                c.registers[x] ^= c.registers[y]
                c.logDebug("[EXECUTE] 8XY3: V%X ^= V%X\n", x, y)
            case 0x4:
                sum := uint16(c.registers[x]) + uint16(c.registers[y])
                if sum > 255 {
                    c.registers[0xF] = 1
                } else {
                    c.registers[0xF] = 0
                }
                c.registers[x] = byte(sum & 0xFF)
                c.logDebug("[EXECUTE] 8XY4: V%X += V%X\n", x, y)
            case 0x5:
                if c.registers[x] >= c.registers[y] {
                    c.registers[0xF] = 1
                } else {
                    c.registers[0xF] = 0
                }
                c.registers[x] -= c.registers[y]
                c.logDebug("[EXECUTE] 8XY5: V%X = V%X - V%X\n", x, x, y)
            case 0x6:
                c.registers[0xF] = c.registers[x] & 0x1
                c.registers[x] >>= 1
                c.logDebug("[EXECUTE] 8XY6: V%X shifted right\n", x)
            case 0x7:
                if c.registers[y] >= c.registers[x] {
                    c.registers[0xF] = 1
                } else {
                    c.registers[0xF] = 0
                }
                c.registers[x] = c.registers[y] - c.registers[x]
                c.logDebug("[EXECUTE] 8XY7: V%X = V%X - V%X\n", x, y, x)
            case 0xE:
                c.registers[0xF] = (c.registers[x] & 0x80) >> 7
                c.registers[x] <<= 1
                c.logDebug("[EXECUTE] 8XYE: V%X shifted left\n", x)
            }

        case 0x9:
            if n == 0x0 {
                if c.registers[x] != c.registers[y] {
                    c.pc += 2
                }
                c.logDebug("[EXECUTE] 9XY0: Skip if V%X != V%X\n", x, y)
            }

        case 0xA:
            c.index = nnn
            c.logDebug("[EXECUTE] ANNN: Index Register assigned to 0x%04X\n", nnn)

        case 0xB:
            c.pc = nnn + uint16(c.registers[0x0])
            pcJumped = true
            c.logDebug("[EXECUTE] BNNN: Jump to 0x%04X + V0\n", nnn)

        case 0xC:
            c.registers[x] = byte(rand.Intn(256)) & nn
            c.logDebug("[EXECUTE] CXNN: Generated random byte into V%X\n", x)

        case 0xD:
            xPos := c.registers[x] % 64
            yPos := c.registers[y] % 32
            var collision byte = 0

            for row := uint16(0); row < uint16(n); row++ {
                currentY := uint16(yPos) + row
                if currentY >= 32 {
                    break
                }
                pixelByte := c.ram[c.index+row]
                for col := uint16(0); col < 8; col++ {
                    currentX := uint16(xPos) + col
                    if currentX >= 64 {
                        break
                    }
                    if (pixelByte & (0x80 >> col)) != 0 {
                        indexDisplay := currentX + (currentY * 64)
                        if c.display[indexDisplay] == 1 {
                            collision = 1
                        }
                        c.display[indexDisplay] ^= 1
                    }
                }
            }
            c.registers[0xF] = collision
            c.logDebug("[EXECUTE] DXYN: Rendered sprite at V%X, V%X with height %d\n", x, y, n)

        case 0xE:
            switch nn {
            case 0x9E:
                key := c.registers[x] & 0xF
                if c.keys[key] {
                    c.pc += 2
                }
                c.logDebug("[EXECUTE] EX9E: Check if key in V%X is pressed\n", x)
            case 0xA1:
                key := c.registers[x] & 0xF
                if !c.keys[key] {
                    c.pc += 2
                }
                c.logDebug("[EXECUTE] EXA1: Check if key in V%X is NOT pressed\n", x)
            }

        case 0xF:
            switch nn {
            case 0x07:
                c.registers[x] = c.delayTimer
                c.logDebug("[EXECUTE] FX07: Read Delay Timer into V%X\n", x)
            case 0x0A:
                keyPressed := false
                for k, pressed := range c.keys {
                    if pressed {
                        c.registers[x] = byte(k)
                        keyPressed = true
                        break
                    }
                }
                if !keyPressed {
                    pcJumped = true
                }
                c.logDebug("[EXECUTE] FX0A: Awaiting keypress into V%X\n", x)
            case 0x15:
                c.delayTimer = c.registers[x]
                c.logDebug("[EXECUTE] FX15: Set Delay Timer to V%X\n", x)
            case 0x18:
                c.soundTimer = c.registers[x]
                c.logDebug("[EXECUTE] FX18: Set Sound Timer to V%X\n", x)
            case 0x1E:
                c.index += uint16(c.registers[x])
                c.logDebug("[EXECUTE] FX1E: Added V%X to Index register\n", x)
            case 0x29:
                c.index = 0x050 + uint16(c.registers[x]&0x0F)*5
                c.logDebug("[EXECUTE] FX29: Point Index to font character in V%X\n", x)
            case 0x33:
                value := c.registers[x]
                c.ram[c.index]     = value / 100
                c.ram[c.index + 1] = (value / 10) % 10
                c.ram[c.index + 2] = value % 10
                c.logDebug("[EXECUTE] FX33: Stored BCD of V%X at I\n", x)
            case 0x55:
                for idx := uint16(0); idx <= uint16(x); idx++ {
                    c.ram[c.index + idx] = c.registers[idx]
                }
                c.logDebug("[EXECUTE] FX55: Dumped V0-V%X into memory starting at I\n", x)
            case 0x65:
                for idx := uint16(0); idx <= uint16(x); idx++ {
                    c.registers[idx] = c.ram[c.index + idx]
                }
                c.logDebug("[EXECUTE] FX65: Loaded V0-V%X from memory starting at I\n", x)
            }
        // prevents a crash by a unkown opcode (handles poor code, a default for JS Vibe Coders)
        default:
            c.logDebug("[WARNING] Opcode pattern 0x%04X is not yet mapped.\n", opcode)
        }

        if !pcJumped {
            c.pc += 2
        }

        if pcJumped {
            break
        }
   }

    return nil
}

// Creates a window and starts the Virtual Green Phosphor CRT monitor
func (c *Chip8) Draw(screen *ebiten.Image) {
    pixelBuffer := make([]byte, 64*32*4)
    for y := 0; y < 32; y++ {
        for x := 0; x < 64; x++ {
            chip8DisplayIndex := x + (y * 64)
            rgbaIndex := (x + (y * 64)) * 4

            if c.display[chip8DisplayIndex] == 1 {
                pixelBuffer[rgbaIndex]   = 0
                pixelBuffer[rgbaIndex+1] = 255
                pixelBuffer[rgbaIndex+2] = 64
                pixelBuffer[rgbaIndex+3] = 255
            } else {
                pixelBuffer[rgbaIndex]   = 0
                pixelBuffer[rgbaIndex+1] = 0
                pixelBuffer[rgbaIndex+2] = 0
                pixelBuffer[rgbaIndex+3] = 255
            }
        }
    }
    screen.WritePixels(pixelBuffer)
}

func (c *Chip8) Layout(outsideWidth, outsideHeight int) (int, int) {
    return 64, 32
}

// Simulates the main bootloader, where you can use any ROM you want
// Remmenber, it's case sensitive, needs a ".ch8" extension and don't even run if that's empty
// Also, this a monotask system, only a single ROM per boot
func (c *Chip8) LoadRom() string {
    path := "./disk_system/"
    os.MkdirAll(path, 0755)

    reader := bufio.NewReader(os.Stdin)
    var fullFilePath string

    for {
        fmt.Print("\n[HAWCHIP-8] Choose the ROM (or type '/exit_program'): ")
        input, _ := reader.ReadString('\n')
        romName := strings.TrimSpace(input)

        // A simple QoL, just in case
        if romName == "/exit_program" {
            fmt.Println("\n[HAWCHIP-8] Execution stopped. Goodbye.")
            os.Exit(0)
        }

        fullFilePath = path + romName + ".ch8"
        fileCheck, err := os.Open(fullFilePath)
        if err == nil {
            fileCheck.Close()
            break
        }
        fmt.Printf("[404 ERROR] file '%s.ch8' not found at '%s'. Try again.\n", romName, path)
    } // The most common case of error it's you have gorilla fingers like me and type "0kng" instead of "Pong"
    // The second one is a broken keyboard type "pONg" or "PPPPPPPPPong"
    // If that is also your case, my condolences.

    file, err := os.Open(fullFilePath)
    if err != nil {
        log.Fatalf("Error opening file: %v", err)
    }
    defer file.Close()

    ramTargetSlice := c.ram[0x200:]
    bytesRead, _ := file.Read(ramTargetSlice)
    fmt.Printf("Loaded %d bytes into RAM successfully!\n", bytesRead)
    
    return fullFilePath
    
} // (don't even think you can Run Discord, Spotify, Chrome and VS Code all at once here, you CRUD Monke-)


// This the end of the code
// The main function that's boot everything in the program
func main() {
    
    // Asks if you want or not the Dev Tool
    reader := bufio.NewReader(os.Stdin)
    fmt.Print("[HAWCHIP-8] Want to activate the Debug Mode (generate .TXT with every instruction of the session)? (y/n): ")
    debugInput, _ := reader.ReadString('\n')
    debugMode := strings.TrimSpace(debugInput) == "y" || strings.TrimSpace(debugInput) == "S" || strings.TrimSpace(debugInput) == "Y"

    chip8 := NewChip8(debugMode)
    chip8.LoadRom()

    ebiten.SetTPS(60) // Sets the update rate of the Update Function
    ebiten.SetWindowSize(640, 320) // Sets the resolution of the screen (standtized at 64 x 32 pixels)
    ebiten.SetWindowTitle("||| <HawCHIP-8> |||") // Just a Eyecandy

    // If something goes wrong, that's the fallback
    if err := ebiten.RunGame(chip8); err != nil {
        fmt.Println("Error while trying to execute the ROM:", err)
    } // If that message appears, pray for mercy.
}

// "Can't fight a battle with your hands on your pocket; Better duck 'cause I'm faster than a Rocket"
// Sorry, I was listen to Accelaracers amazing soundtrack (Tip: Search by "Go (teku)")
