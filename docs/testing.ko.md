# 테스트와 커버리지

[English](testing.md) | [한국어](testing.ko.md)

필수 검사인 **Quality gate**는 모든 OS·Go 조합 테스트, 정적 검사, 커버리지, 퍼징, 스트레스 검사와 배포 파일 검증에 의존합니다. 하나라도 실패하거나 생략되면 gate가 실패합니다. 릴리스도 태그 커밋에서 같은 CI를 먼저 실행합니다.

## 검사 범위

| 검사 | 범위 |
|---|---|
| 호환성 | Ubuntu·macOS·Windows × Go 1.25.x·1.26.x·1.27.x |
| 회귀 | 전체 Go 테스트 `-race -shuffle=on -count=3` |
| 통합 | 패치 생성 → Git 적용 → 실제 Go 컴파일·race 검사 및 CLI 실행 |
| 패키지 | go list 패턴, 중복 입력, build tag, 중첩 모듈, 파일, `-C` |
| 파일 시스템 | 미리보기 무수정, 권한 보존, 변경된 원본·심볼릭 링크 거부 |
| 스트레스 | 핵심 회귀 `-race -count=50 -cpu=1,2,4 -shuffle=on` |
| 퍼징 | 임의 소스와 생성 패치 왕복 검사 각각 60초 |
| 정적 검사 | gofmt, go vet, 버전 고정 actionlint |
| 보조 스크립트 | 가중 커버리지, 잘못된 profile, 패키지별 기준, 태그, 재현 가능한 압축 |
| 배포 | 6개 아카이브·manifest·SHA-256 및 호스트 실행 파일 version/help |

스트레스 검사는 통합 fixture를 150번 재빌드하는 대신 핵심 회귀에 집중합니다. 전체 통합 검사는 모든 OS·Go 조합에서 실행합니다. 퍼징은 임의 생성된 Go 소스를 실행하지 않으며 실패 재현 자료를 아티팩트로 보존합니다.

## 커버리지

`python scripts/quality.py coverage`는 다음 검사를 실행합니다.

```sh
go test -race -covermode=atomic -coverpkg=./... -coverprofile=coverage/coverage.out -shuffle=on -count=1 -timeout=5m ./...
```

전체 statement coverage **95%**, 각 Go 패키지 **90%**를 요구합니다. Go 패키지나 생성된 제품 소스를 제외하지 않습니다. 블록 중복을 제거하고 문장 수로 가중 계산하며, 비어 있거나 잘못되거나 모순된 profile은 실패 처리합니다.

`coverage-report` 아티팩트에는 줄별 HTML 뷰어 `index.html`, 원본 `coverage.out`, 함수별 `functions.txt`, 영어·한국어 요약 `summary.md`·`summary.ko.md`, 수치·기준·소스 지문·Go 버전이 담긴 `summary.json`, 배지 `coverage.svg`가 있습니다. HTML은 다운로드 후 브라우저에서 여세요.

영문 요약은 Actions Job Summary에도 표시됩니다. README 배지는 [커밋된 스냅샷](coverage.ko.md)이며 `make coverage` 또는 `python scripts/quality.py coverage --snapshot`으로 갱신합니다.

Statement coverage는 분기·mutation coverage나 전환 의미의 안전성을 입증하지 않습니다. CLI 진입점은 별도 프로세스로 검증하여 부모 profile에는 한 줄의 `os.Exit`가 집계되지 않을 수 있습니다. 일부 예외적인 파일 시스템 실패 경로도 미측정입니다.

## 로컬 재현

```sh
python3 scripts/quality.py lint
python3 scripts/quality.py test
python3 scripts/quality.py coverage --snapshot
python3 scripts/quality.py stress
python3 scripts/quality.py fuzz --fuzztime=60s
```

Go 1.25 이상, Git, Python 3.12 이상이 필요합니다. Go·Python 테스트에 외부 라이브러리는 없으며 워크플로 검사는 CI에 고정된 actionlint를 추가로 사용합니다.

초기 로컬 검증은 macOS ARM64의 Go 1.25.0·1.27.1 race 검사, 각 30초 퍼징, 6개 아카이브 빌드와 macOS ARM64 실행 검사로 진행했습니다. 다른 조합의 실제 실행 상태는 GitHub Actions에서 확인하세요.
