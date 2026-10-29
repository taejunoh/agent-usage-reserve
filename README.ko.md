# Agent Usage Reserve

**Codex나 Claude 요금제 사용량의 일부를 예약 작업용으로 남겨 둡니다.**

설치하면 Codex와 Claude가 일반 작업을 시작하기 전과 큰 단계 사이마다 남은 사용량(5시간 한도와 주간 한도)을 확인합니다. 남은 양이 10% 이하가 되면 진행 상황을 저장하고 멈춘 뒤 다음을 알려 줍니다.

- 어느 한도가 얼마나 남았는지
- 지금까지 끝낸 작업
- 다시 이어서 하는 방법

예약 작업은 계속 실행되므로, 남은 사용량을 예약 작업에 쓸 수 있습니다. 지금 작업을 그래도 끝내고 싶다면 에이전트에게 계속하라고 말하면 됩니다.

[English](README.md)

![격리된 데모 프로필에서 실행한 실제 CLI 출력](docs/assets/demo.gif)

두 에이전트가 모두 있는 격리된 데모 프로필에서 실행한 실제 CLI 출력입니다. 임시 경로는 줄여서 표시했습니다.

> 에이전트가 따르는 지침이며, 계정 사용량을 강제로 막지는 않습니다. 모두 내 컴퓨터 안에서만 동작합니다. 계정 로그인, API 키, 서버, 추적이 없습니다.

## 어디서 동작하나요

| 에이전트 | 동작하는 곳 | 참고 |
| --- | --- | --- |
| **Claude** | Claude 데스크톱 앱의 **Code** 탭 | 터미널의 `claude` 명령에는 사용량을 읽는 방법이 없어서, 그곳에서는 사용량을 확인할 수 없다고만 알립니다. |
| **Codex** | 사용량 한도를 보여 주는 Codex 앱과 환경 | 일부 명령줄 실행이나 예약 실행에서는 사용량이 보이지 않을 수 있습니다. |

## 설치

아래 중 **하나만** 고르세요.

**Mac 또는 Linux, 명령 하나로** (설치하고 바로 켭니다):

```sh
curl -fsSL https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.sh | sh
```

**Windows**, PowerShell에서 (설치하고 바로 켭니다):

```powershell
irm https://raw.githubusercontent.com/taejunoh/agent-usage-reserve/main/install.ps1 | iex
```

Mac 또는 Linux에서 **Homebrew**를 쓴다면 (명령 두 개: 설치한 뒤 켜기):

```sh
brew install taejunoh/tap/agent-usage-reserve
agent-usage-reserve enable
```

### 설치할 때 일어나는 일

- Codex나 Claude 중 하나만 쓰면, 그쪽에 10%로 켜집니다.
- 둘 다 쓰면 한 번 물어봅니다.

  ```text
  Found Codex and Claude. Apply the usage reserve to:
    1) Both
    2) Claude only
    3) Codex only
  Choose [1]:
  ```

  Enter를 누르면 둘 다 켜집니다.
- 설치기가 물어볼 수 없는 상황(예: 자동 설정)이면 아무것도 바꾸지 않고, 대신 실행할 명령(예: `agent-usage-reserve enable --only all`)을 알려 줍니다.
- Codex와 Claude가 둘 다 아직 설치되지 않았다면 아무것도 바꾸지 않습니다. 둘 중 하나를 먼저 설치한 뒤 `agent-usage-reserve enable`을 실행하세요.

설치기는 내려받은 파일의 체크섬을 확인하고, 내 사용자 폴더에만 설치하며(Mac/Linux는 `~/.local/bin`, Windows는 `%LOCALAPPDATA%\Programs\agent-usage-reserve\bin`), 관리자 비밀번호를 묻지 않습니다. 설치기가 `PATH`에 관한 줄을 출력하면 그대로 따라 하세요. 그래야 `agent-usage-reserve`를 바로 입력해 쓸 수 있습니다.

## 자주 쓰는 명령

| 하고 싶은 일 | 실행할 명령 |
| --- | --- |
| 무엇이 켜져 있는지 보기 | `agent-usage-reserve status` |
| 켜기 (또는 다른 에이전트 추가) | `agent-usage-reserve enable` |
| 남겨 둘 양을 15%로 바꾸기 | `agent-usage-reserve set 15` |
| 끄기 | `agent-usage-reserve disable` |
| 버전 보기 | `agent-usage-reserve --version` |

남겨 둘 양은 1부터 99까지의 정수이고, Codex와 Claude에 따로 정할 수 있습니다. 한쪽만 바꾸려면 `--only claude`나 `--only codex`를 붙이세요. 예: `agent-usage-reserve set 20 --only codex`

`status`는 지침 파일에 무엇이 설치돼 있는지만 보여 줍니다. 실제 계정 사용량을 조회하지는 않습니다.

## 업데이트

설치할 때 쓴 명령을 다시 실행하세요. 지금 설정은 그대로 유지됩니다.

Homebrew라면:

```sh
brew update
brew upgrade taejunoh/tap/agent-usage-reserve
```

## 삭제

1. 먼저 끕니다: `agent-usage-reserve disable`
2. 프로그램을 지웁니다: Homebrew라면 `brew uninstall taejunoh/tap/agent-usage-reserve`, 아니면 위 설치 폴더에서 `agent-usage-reserve` 파일을 삭제합니다.

1번을 먼저 하세요. 프로그램만 지우면 지침 파일에 설정이 그대로 남습니다.

## codex-usage-reserve에서 넘어오는 경우

이 도구의 예전 이름은 `codex-usage-reserve`였고 Codex만 지원했습니다. 이제 Claude도 지원합니다.

1. 설치했던 방법 그대로 업데이트하세요.
   - **Homebrew:** `brew update` 후 `brew upgrade taejunoh/tap/agent-usage-reserve`. Homebrew가 예전 이름을 새 이름으로 자동으로 옮깁니다.
   - **설치 명령:** 위의 새 설치 명령을 실행하세요. 예전 설치 명령도 계속 동작합니다. 어느 쪽이든 설치하는 폴더에 있는 예전 `codex-usage-reserve` 프로그램을 지우고, 다른 곳에 사본이 남아 있으면 알려 줍니다.
2. `agent-usage-reserve enable`을 한 번 실행하세요. Codex 설정은 그대로 두고, Claude도 쓰고 있다면 `Also enable for Claude? [Y/n]`라고 묻습니다.

기존 Codex 설정은 그동안에도 계속 동작합니다.

## 동작 방식과 바뀌는 것

에이전트마다 지침 파일 하나에 표시된 구역 하나만 추가하고, 다른 내용은 건드리지 않습니다.

- **Claude:** Claude 폴더(`~/.claude`, 또는 `CLAUDE_CONFIG_DIR`을 설정했다면 그 경로)의 `CLAUDE.md`
- **Codex:** Codex 폴더(`~/.codex`, 또는 `CODEX_HOME`을 설정했다면 그 경로)의 `AGENTS.md`. 내용이 있는 `AGENTS.override.md`가 있으면 Codex가 그 파일을 먼저 읽으므로 그 파일을 씁니다.

바꾸기 전마다 같은 폴더에 원본 파일의 비공개 백업을 저장합니다. `disable`은 자기 구역만 지우고 나머지 내용은 원래 그대로 둡니다.

정책 세부 사항:

- 예약 작업 실행은 예외입니다. 일반 수동 작업은 남겨 둘 양에서 멈춥니다.
- "계속"은 지정한 작업(지정하지 않으면 현재 작업)에만 적용됩니다. 그 작업에 더 낮은 임시 기준을 줄 수도 있습니다. 예: "3%까지 계속해"
- 서브에이전트는 확인을 건너뛰고, 메인 세션이 처리합니다.
- 사용량을 읽을 수 없으면 추측하지 않고 그렇다고 알립니다.

## 고급

### 옵션

```text
agent-usage-reserve enable  [--threshold 10] [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve set 15  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve status  [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve disable [--only claude|codex|all] [--codex-home PATH] [--claude-home PATH]
agent-usage-reserve --version
```

- `--only`는 묻지 않고 에이전트를 고릅니다. `--codex-home` / `--claude-home`은 다른 폴더를 가리키며, 그 에이전트를 고른 것으로도 칩니다.
- 에이전트 폴더가 있으면 그 에이전트를 찾은 것으로 봅니다. 명시적으로 고른 에이전트의 폴더가 없으면 `enable`이 만듭니다.
- `enable`을 다시 실행하면 이미 켜진 에이전트와 그 기준을 유지합니다. `--threshold`를 주면 기준을 바꿉니다.
- `--only` 없이 `set`과 `disable`을 실행하면 켜진 모든 에이전트에 적용됩니다.
- 선택이 필요한데 물어볼 터미널이 없거나, 에이전트를 하나도 찾지 못하면 `enable`은 아무것도 바꾸지 않고 종료 코드 3으로 끝납니다.
- 파일을 바꾸기 전에 작업 중 다른 무언가가 그 파일을 바꿨는지 확인합니다. 외부 편집 대부분을 잡아내지만, 동시에 일어나는 모든 쓰기를 잡아내지는 못합니다.

### 백업 복원

구역을 지우려면 `agent-usage-reserve disable`을 쓰세요. 파일 전체를 예전 상태로 되돌려야 할 때만 백업을 복원하세요.

백업 파일 이름은 `.agent-usage-reserve-backup-*`이고 각 에이전트 폴더에 남습니다. 이름이 바뀌기 전에 만든 백업은 `.codex-usage-reserve-backup-*`이며 그대로 보존됩니다.

전체 복원은 파일 전체를 바꾸므로, 그 백업 이후에 고친 내용은 사라집니다. 백업은 그 백업이 들어 있는 폴더의 파일에만 복원하세요. Codex는 `status`에 나온 파일(`AGENTS.md` 또는 `AGENTS.override.md`)을 대상으로 합니다.

Mac/Linux:

```sh
codex_home=${CODEX_HOME:-"$HOME/.codex"}
claude_home=${CLAUDE_CONFIG_DIR:-"$HOME/.claude"}
find "$codex_home" "$claude_home" -type f -name '.agent-usage-reserve-backup-*' -print
```

```sh
backup='/full/path/to/.agent-usage-reserve-backup-...'
target="$codex_home/AGENTS.md" # AGENTS.override.md가 활성 Codex 파일이면 그 경로 사용
# Claude 백업이라면: target="$claude_home/CLAUDE.md"
diff -u "$target" "$backup" || true
cp -p "$target" "$target.before-restore"
cp -p "$backup" "$target" # 전체 파일 복원을 의도한 경우에만 실행
```

Windows PowerShell:

```powershell
$codexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME '.codex' }
$claudeHome = if ($env:CLAUDE_CONFIG_DIR) { $env:CLAUDE_CONFIG_DIR } else { Join-Path $HOME '.claude' }
Get-ChildItem -Force -File $codexHome, $claudeHome -Filter '.agent-usage-reserve-backup-*' |
  Sort-Object LastWriteTime -Descending
```

```powershell
$backup = 'C:\full\path\to\.agent-usage-reserve-backup-...'
$target = Join-Path $codexHome 'AGENTS.md' # AGENTS.override.md가 활성 Codex 파일이면 그 경로 사용
# Claude 백업이라면: $target = Join-Path $claudeHome 'CLAUDE.md'
Get-FileHash -LiteralPath $backup, $target
Copy-Item -LiteralPath $target -Destination "$target.before-restore"
Copy-Item -LiteralPath $backup -Destination $target -Force # 전체 파일 복원을 의도한 경우에만 실행
```

나중에 고친 내용을 지키려면 파일 전체 대신 되돌리고 싶은 부분만 옮기세요.

## 개발자용

출시된 프로그램에는 Go, Node, Python, API 키가 필요 없습니다. 소스에서 빌드하려면 Go 1.23 이상이 필요합니다.

```sh
mkdir -p "$HOME/.local/bin"
go build -o "$HOME/.local/bin/agent-usage-reserve" ./cmd/agent-usage-reserve
"$HOME/.local/bin/agent-usage-reserve" enable
```

`$HOME/.local/bin`이 `PATH`에 없다면 셸 설정에 직접 추가하세요.

릴리스를 준비하려면 `scripts/build-release.sh 0.2.0`으로 6개 플랫폼 아카이브와 SHA-256 항목을 `dist/`에 만듭니다. `v*` 태그를 푸시하면 릴리스 워크플로가 이를 게시합니다. 설치 명령은 `main`과 최신 릴리스를 함께 읽기 때문에, 커밋이 이미 `main`에 있을 때만 릴리스를 Latest로 지정합니다.

Homebrew formula는 tap 저장소를 따로 체크아웃한 곳에서 릴리스 체크섬으로 생성합니다. `--renames`는 예전 formula 이름을 새 이름으로 연결하는 `formula_renames.json`도 함께 만듭니다.

```sh
python3 scripts/generate-formula.py 0.2.0 dist/checksums.txt ../homebrew-tap/Formula/agent-usage-reserve.rb --renames ../homebrew-tap/formula_renames.json
```

생성기는 6개 아카이브의 실제 체크섬을 모두 요구합니다. 생성된 formula는 tap 저장소에서 검토한 뒤 게시하세요.

데모 GIF와 트랜스크립트를 다시 만들려면 바이너리를 빌드한 뒤 스크립트를 실행합니다(Pillow가 있는 Python과 고정폭 글꼴이 필요합니다).

```sh
go build -o /tmp/agent-usage-reserve ./cmd/agent-usage-reserve
AGENT_USAGE_RESERVE_DEMO_BINARY=/tmp/agent-usage-reserve python3 scripts/generate-demo.py
```
