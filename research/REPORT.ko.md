# synctest 전환 도구: 공개 저장소 실험

[English](REPORT.md) | [한국어](REPORT.ko.md)

2026-09-05, Go 1.27.1, macOS ARM64에서 수행했습니다.

**기술적 효용은 확인했습니다.** 고정한 공개 저장소 3개, 테스트 파일 17개에서
직접적인 시간 관련 후보 6개를 찾았습니다. 그중 한 저장소의 테스트 3개를
CLI로 전환하고 전후 실행을 검증했습니다. 유지보수자의 채택 의사나 시장 수요는
아직 확인하지 않았습니다.

## 후보 검색

| 저장소 | 고정 커밋 | 테스트 파일 | 직접 탐지한 후보 | 판단 |
|---|---|---:|---:|---|
| cenkalti/backoff | [ffcfd8ab39e2](https://github.com/cenkalti/backoff/tree/ffcfd8ab39e2910a1180ba0b7a02a52f0485adc9) | 6 | 1 | go.mod가 1.23; 최소 지원 버전 결정 필요 |
| avast/retry-go | [5bccbfa9340df](https://github.com/avast/retry-go/tree/5bccbfa9340dfe6609f4ecfce30c971e2756c796) | 6 | 1 | go.mod가 1.20; 최소 지원 버전 결정 필요 |
| sethvargo/go-retry | [f6b3e1a9f1c5](https://github.com/sethvargo/go-retry/tree/f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502) | 5 | 4 | go.mod가 1.25; 아래 3개를 선택 |

원시 목록: [backoff](scans/backoff.json), [retry-go](scans/retry-go.json),
[go-retry](scans/go-retry.json).

후보 수는 전체 전환 가능 테스트의 수가 아닙니다. 이 프로토타입은 동적인 이름의
table-driven subtest와 helper 안의 타이머를 놓칩니다. 예를 들어 go-retry의
일부 backoff 테스트는 `t.Run(tc.name, ...)` 아래에 타이머가 있어 후보에 포함되지
않았습니다. 반대로 `TestExponentialBackoff_ConcurrentOverflow`의 `time.After`
는 실패 시의 안전장치여서, 정상 실행 시간 단축 효과가 작다고 판단해 제외했습니다.

## 측정 결과

각 테스트를 **20회 반복하는 프로세스를 3회** 실행했습니다. 표는 3회 배치 실행의
중앙값이며, 매 trial마다 변경 전/후를 번갈아 실행했습니다. 동일한 Go 버전으로
미리 컴파일한 테스트 바이너리를 사용했습니다. 컴파일 시간은 제외하고 프로세스
시작 비용은 포함했습니다. 성능 측정에는 race detector를 사용하지 않았습니다.

| 선택한 테스트 | 변경 전, 20회 | 변경 후, 20회 | 배치 실행 시간 비율 |
|---|---:|---:|---:|
| TestWithMaxDuration | 5.050초 | 0.0126초 | 약 401배 |
| TestDo/context_canceled | 1.039초 | 0.0103초 | 약 101배 |
| TestDo/deadline_exceeded | 0.227초 | 0.0131초 | 약 17배 |

이 수치는 실제 기다림을 없앤 **선택된 테스트**의 결과입니다. 전체 프로젝트나
CI가 이 비율로 빨라진다는 뜻은 아닙니다. 첫 번째 변경 후 배치는 0.558초였고
나머지는 0.0064초, 0.0126초였습니다. 시작 비용과 시스템 상태의 영향이 크므로
수치를 마이크로벤치마크로 해석하면 안 됩니다. 모든 원시 측정값을 보존했습니다.

- [원시 시간·커밋·환경](results.json)
- [반복 실행 로그](logs/)
- [backoff 전환 패치](patches/go-retry-backoff.patch)
- [context 전환 패치](patches/go-retry-context.patch)

## 동작 검증

- 선택된 3개 테스트 각각 변경 전 60회, 변경 후 60회 통과.
- 변경 전/후 모두 `go test -race -count=10 -timeout=60s ./...` 통과.
- 실제 파일이 바뀌었고, 선택한 테스트 수만큼 `synctest.Test`가 들어갔는지 확인.
- `WithMaxDuration`의 만료 조건 `diff <= 0`을 의도적으로 `diff < 0`으로 바꾸자
  전환된 테스트가 `should stop`으로 실패. 정확히 250ms인 경계의 잘못된 동작을
  검출했습니다. 이 mutation은 임시 사본에서만 적용하고 복원했습니다.

[변경 전 race 로그](logs/before-race.txt), [변경 후 race 로그](logs/after-race.txt),
[mutation 검출 로그](logs/mutation.txt).

반복 테스트가 통과한 것으로 원래 테스트의 flaky 빈도 감소나 모든 동작의
등가성을 증명하지는 않았습니다. 측정 중 기존 테스트의 실패도 관찰하지 않았습니다.

## 수동으로 확인한 전환 근거

`TestWithMaxDuration`은 함수 안에서 backoff 상태를 만들고, `time.Now`와
`time.Since`를 사용하는 `WithMaxDuration`을 호출합니다. 외부 I/O가 없으며
200ms, 50ms 대기가 로컬 상태의 만료를 검사하므로 가상 시간 전환 목적이 명확합니다.

`TestDo/context_canceled`는 로컬 context와 순수 콜백을 만들고, `Do` → `DoValue`의
timer/context select를 통해 종료합니다. HTTP 코드가 같은 파일의 예제에 있지만
이 테스트 경로에서는 호출하지 않습니다. 파일에 `net/http` import가 있다는
이유만으로 테스트를 제외하면 이 후보를 놓칩니다.

`TestDo/deadline_exceeded`도 같은 경로를 사용합니다. 다만 원본의 `time.Since`
assertion은 실제 경과 시간을 재며, 전환 후에는 가상 시간을 잽니다. 전환 후에는
**context 만료와 backoff의 논리적 순서**를 검증하고, 실제 스케줄러 지연에 대한
성능 검증은 하지 않습니다. 유지보수자가 원했던 계약이 무엇인지에 따라 이 세 번째
전환은 별도로 판단해야 합니다.

세 테스트의 첫 `t.Parallel()`은 그대로 bubble 밖에 남깁니다. backoff 테스트의
독립된 `time.Sleep` 뒤에만 `synctest.Wait()`를 추가했습니다. go.mod는 바꾸지
않았습니다. 공식 제약: [testing/synctest](https://pkg.go.dev/testing/synctest).

## 다음 제품 판단

작은 CLI로 가치를 보여줄 수 있다는 가설은 통과했습니다. 하지만 현재의
`review` 분류는 자동 전환을 보장하는 수준이 아닙니다. 다음 개선 우선순위는:

1. `go/packages` 및 타입 정보를 사용해 helper와 로컬 호출 경로를 추적하기.
2. 시간을 단순히 기다리는 테스트와 실제 지연을 검증하는 테스트를 구분하기.
3. 동적인 table-driven subtest를 다루기.
4. 추가 저장소에서 제안 패치를 검토받아 채택 여부를 확인하기.

초기 고객 가설은 이미 Go 1.25 이상을 쓰는 팀입니다. 구버전 호환성을 유지하는
라이브러리에는 절약 시간보다 최소 Go 버전 상승 비용이 더 클 수 있습니다.

GitHub issue나 PR은 게시하지 않았습니다. [유지보수자용 초안](PR_DRAFT.md)은
검토 가능한 로컬 문서입니다.

## 재현

프로젝트 루트에서 `python3 research/experiment.py`를 실행합니다. 고정 커밋을
다운로드하고 독립된 Git 루트를 가진 임시 사본에서 패치·테스트·측정을 수행합니다.
생성 파일은 `research/`에, 원본 저장소 캐시는 `.research/repos/`에 남습니다.

패치에 포함된 go-retry 원본 코드의 라이선스는 Apache-2.0이며,
[원본 라이선스 사본](licenses/go-retry-APACHE-2.0.txt)을 함께 보존했습니다.
