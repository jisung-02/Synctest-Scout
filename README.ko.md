# Synctest Scout

[English](README.md) | [한국어](README.ko.md)

[![CI](https://github.com/jisung-02/Synctest-Scout/actions/workflows/ci.yml/badge.svg)](https://github.com/jisung-02/Synctest-Scout/actions/workflows/ci.yml)

[![Coverage snapshot](docs/coverage.svg)](docs/coverage.ko.md)

Go 테스트의 실제 시간 대기를 찾아 `testing/synctest`로 옮기는 CLI입니다.
Go 표준 도구와 비슷하게 패키지를 선택하고, diff를 확인한 뒤 수정합니다.

```sh
# 이 저장소에는 go.mod의 tool 항목이 설정되어 있습니다.
go tool synctest-scout help
go tool synctest-scout -C /path/to/project scan ./...
go tool synctest-scout -C /path/to/project fix -diff -run '^TestWithMaxDuration$' ./...

# diff와 호출 경로를 검토한 뒤 적용
go tool synctest-scout -C /path/to/project fix -run '^TestWithMaxDuration$' ./...
```

`fix -diff`는 **변경이 있으면 종료 코드 1**, 없으면 0을 반환합니다.
`go fix -diff`와 같은 방식이며, diff가 나왔다는 이유만으로 오류 메시지를 내지 않습니다.
`fix`의 기본 동작은 파일 수정입니다. 먼저 `-diff`로 전환 내용을 확인하세요.

## 설치

Go 1.25 이상과 Git이 필요합니다. 외부 Go 모듈 의존성은 없습니다.

```sh
go install github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest
synctest-scout version
```

다른 Go 모듈에 도구 의존성으로 등록할 수 있습니다.

```sh
go get -tool github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest
go tool synctest-scout scan ./...
```

이 저장소 자체에는 `tool` 항목이 설정되어 있습니다.

```sh
git clone https://github.com/jisung-02/Synctest-Scout.git
cd Synctest-Scout
go tool synctest-scout help
```

버전 태그는 Linux·macOS·Windows amd64/arm64 아카이브를 생성합니다.
[배포 안내](docs/releasing.ko.md)와 [GitHub Releases](https://github.com/jisung-02/Synctest-Scout/releases)를 참고하세요. 첫 태그 이전에도 소스 설치가 가능합니다.

## 명령과 옵션

| 명령 / 옵션 | 동작 |
|---|---|
| `help`, `help scan`, `help fix` | 전체 도움말 또는 명령별 옵션 |
| `version` | 버전, 커밋, 빌드 메타데이터 |
| `-C dir` | 해당 디렉터리를 기준으로 실행; 명령 앞에 사용 |
| `scan [packages]` | 시간 관련 테스트와 검토 이유 출력 |
| `scan -json [packages]` | 구조화된 JSON 리포트 |
| `fix [packages]` | 후보를 전환하고 변경한 파일 이름 출력 |
| `fix -diff [packages]` | 수정 없이 unified diff 출력 |
| `fix -n [packages]` | 수정 없이 변경할 파일 이름만 출력 |
| `fix -run regexp [packages]` | 전체 테스트 경로에 정규식을 적용해 선택 |
| `scan/fix -tags a,b` | Go build tag에 맞춰 파일 선택 |

패키지 기본값은 `.`입니다. `./...`, `./internal/...`, 여러 패키지, 명시적인
`*_test.go` 파일을 사용할 수 있습니다. 플래그는 패키지 인수 앞에 둡니다.
`-run`은 `TestDo/context_canceled`처럼 슬래시로 이어진 전체 경로에 대한
정규식이며, `go test`의 단계별 subtest 필터 구현과 완전히 같지는 않습니다.

패키지는 `go list -mod=readonly`로 해석하므로 현재 플랫폼·build tag·중첩 모듈
경계를 따릅니다. 대상 테스트 자체를 실행하지는 않지만, Go 명령이 모듈 캐시나
프록시에 접근할 수 있습니다. `go.mod`와 `go.sum`을 수정하지 않습니다.

## 전환 범위

- `time.Sleep`, 타이머·티커, context timeout/deadline, 일부 testify Eventually 탐지.
- import 별칭과 로컬 변수에 가려진 패키지 이름 구별.
- 최상위 테스트와 이름이 문자열 리터럴인 leaf subtest 선택.
- 첫 문장의 `t.Parallel()`은 bubble 바깥에 유지.
- 독립된 `time.Sleep` 문장 뒤에 `synctest.Wait()` 추가.
- 최소 Go 버전, 직접적인 외부 I/O, 기존 synctest 등은 수동 검토로 분류.
- 수정 전 모든 파일을 준비하고 원본 변경 여부 확인. 파일별 임시 파일·rename으로
  교체하며 권한을 보존합니다. 여러 파일 전체에 대한 트랜잭션은 아닙니다.
- 기준 디렉터리 바깥의 파일 수정은 거부합니다. 프로젝트 루트를 `-C`로 지정하세요.

`review`는 직접 보이는 차단 요인이 없다는 의미이고, `manual`은 자동 수정에서
제외한다는 의미입니다. **함수 호출 그래프나 의미적 안전성을 증명하지 않습니다.**
외부 I/O, 공유 상태, goroutine 수명, 실제 시간 측정이 필요한 assertion은
직접 검토해야 합니다. `Wait` 추가 역시 동기화 순서에 영향을 줄 수 있습니다.

helper 내부의 시간 호출, 동적으로 이름이 정해지는 table-driven subtest는
놓칠 수 있습니다. 파일에 `testing/synctest` import가 이미 있으면 추가 전환은
수동으로 처리합니다. 모든 동시성 테스트가 자동 전환 대상은 아닙니다.

이전 프로토타입의 `patch -file FILE -test NAME -reviewed`도 호환용으로
지원합니다. 새 작업에는 `fix -diff -run ...`을 사용하세요.

## CI와 커버리지

[커버리지 리포트](docs/coverage.ko.md) · [CI 설정](.github/workflows/ci.yml) ·
[검증 방법](docs/testing.ko.md)

- **Go 1.25 / 1.26 / 1.27 × Linux / macOS / Windows** 9개 조합.
- 전체 테스트 `-race -shuffle=on -count=3`.
- 핵심 회귀 테스트 50회 × CPU 1·2·4로 반복.
- 입력 소스 및 패치 적용 왕복 퍼징 각각 60초.
- 실제 diff 적용 → 별도 Go 모듈 컴파일 → race 검사를 포함한 통합 테스트.
- 전체 statement coverage **95%**, 각 패키지 **90%** 미만이면 실패.
- `gofmt`, `go vet`, Actions 문법 검사, 리포트·배포 스크립트 테스트.
- 6개 배포 아카이브 생성과 체크섬·호스트 실행 파일 검증.

CI의 `coverage-report` 아티팩트에는 HTML 줄별 리포트, 원본 profile,
함수별 결과, JSON 요약과 SVG 배지가 들어갑니다. Actions 실행 화면의
Job Summary에서도 결과를 확인할 수 있습니다.

README 배지는 **커밋된 측정 스냅샷**입니다. 최신 CI 상태를 가장하는 배지가
아니며, 매 실행의 최신 결과는 Job Summary와 아티팩트에 생성됩니다.

```sh
make lint
make test
make coverage   # HTML 리포트 및 README용 리포트/배지 갱신
make stress
make fuzz
```

Windows에서는 Make 대신 `python scripts/quality.py test`처럼 직접 실행할 수
있습니다. Python 3.12 이상을 사용하며 별도 Python 패키지는 필요 없습니다.

## 공개 저장소 실험

[실험 결과](research/REPORT.ko.md) · [원시 측정](research/results.json) ·
[생성 패치](research/patches)

초기 프로토타입으로 공개 저장소 3개를 조사하고 `sethvargo/go-retry` 테스트
3개를 전환했습니다. 250ms 대기 테스트의 20회 배치 중앙값은
**5.05초 → 0.013초**로 줄었습니다. 전체 CI 속도에 대한 수치는 아닙니다.

```sh
# 공개 저장소 코드를 임시 사본에서 실제로 실행하는 별도 실험입니다.
python3 research/experiment.py
```

일반 CI는 네트워크 상태나 외부 저장소 변화에 의존하는 이 실험 대신, 저장소 안의
통합 테스트로 도구 자체를 검증합니다.

공식 동작 참고: [testing/synctest](https://pkg.go.dev/testing/synctest).

## 기여와 지원

[기여 안내](CONTRIBUTING.ko.md), [지원](SUPPORT.ko.md), [행동 강령](CODE_OF_CONDUCT.ko.md), [보안 정책](SECURITY.ko.md)을 참고하세요.

영어가 기본 문서 언어입니다. 한국어 번역은 같은 이름의 `.ko.md`로 제공하고 상단에서 상호 연결합니다. 문서화된 동작이 바뀌면 두 언어를 함께 갱신합니다.

## 라이선스

원본 코드는 [MIT 라이선스](LICENSE)로 제공됩니다. 실험 패치의 Apache-2.0 조건은 [외부 자료 고지](THIRD_PARTY_NOTICES.ko.md)를 참고하세요.
