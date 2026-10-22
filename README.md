# 👾 HawCHIP-8

A custom **CHIP-8 Virtual Machine / Interpreter** built from scratch.
Devloped for lonelly Asssembly programmers (A.K.A: Me) fell like programming in the Apple II
(but without have an Insanity debuff programming in the MOS 6502)

<video src="assets/gameplay.mp4" autoplay loop muted playsinline width="25%"></video>

---

## ✨ Features

- **Full Instruction Set:** Implements all 35 standard CHIP-8 opcodes.
- **Display:** 64x32 monochrome display matrix with smooth rendering.
- **Timers:** 60Hz Delay and Sound timers.
- **Keyboard Mapping:** Configured to map modern QWERTY keys to the classic 16-key hexadecimal keypad.

  (Everything would you have expect from a Default CHIP-8)

---

## 🕹️ Controls Mapping

HawCHIP-8 maps the original 4x4 keypad to the QWERTY keyboard:

| CHIP-8 Keypad | Keyboard Input |
| :---: | :---: |
| `1` `2` `3` `C` | `1` `2` `3` `4` |
| `4` `5` `6` `D` | `Q` `W` `E` `R` |
| `7` `8` `9` `E` | `A` `S` `D` `F` |
| `A` `0` `B` `F` | `Z` `X` `C` `V` |

---

## 🚀 Getting Started

### Prerequisites
- Go (language) installed on machine
- Ebitengine (video & audio)

### Build & Run
```bash
# Clone the repository
git clone git@github.com: Hawthrone-LK/hawchip-8.git

# Navigate into the project
cd hawchip-8

# Build the project
go run main.go

# And then, it's alive!
```

---

## 🛠️ Tech Stack & Architecture

- **Language:** Go (unfortunally, has to be Go or x86_64 Assembly...)
- **Graphics / Audio Library:** Ebitengine
- **Memory Map:** 4096 bytes (0x000 - 0xFFF)
- **Registers:** 16 8-bit general-purpose registers (V0-VF)


_ And I'm without any ideas how to write this doc... Want I cand told you that you wouldn't know about it?_
