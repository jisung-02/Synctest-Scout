# CLI 릴리스

[English](releasing.md) | [한국어](releasing.ko.md)

Synctest Scout는 GitHub Releases에서 배포하는 CLI이며 별도 서버 배포는 필요 없습니다. `v*` 태그를 푸시하면 해당 커밋에서 전체 CI를 실행한 뒤 배포합니다. 빌드·테스트 권한은 읽기 전용이며 마지막 게시 작업만 `contents: write`를 가집니다.

## 로컬 패키징

```sh
python3 scripts/release.py --version v0.0.0-dev --output dist/snapshot
python3 scripts/smoke_release.py dist/snapshot
```

이전 파일이 섞이지 않도록 출력 디렉터리는 비어 있어야 합니다. 재빌드는 새 디렉터리를 선택하세요.

Linux·macOS·Windows의 amd64·arm64용으로 다음을 생성합니다.

- Linux·macOS tar.gz 및 Windows zip 아카이브.
- 영문·한국어 문서, MIT 라이선스와 외부 자료 고지.
- `CGO_ENABLED=0`, `-trimpath`, 버전·커밋 정보가 적용된 CLI.
- 플랫폼, 크기, SHA-256과 도구 버전을 담은 `manifest.json`.
- 아카이브와 manifest의 `checksums.txt`.

Go와 Python 표준 라이브러리로 빌드합니다. 압축 파일의 시간·권한·소유자 정보를 고정하여 같은 바이너리 입력에서 같은 아카이브를 만듭니다. Go 버전 사이의 바이너리 동일성을 보장하지는 않습니다. CLI의 diff에는 Git, 패키지 탐색에는 Go가 필요합니다.

Smoke 검사는 모든 체크섬과 6개 플랫폼 manifest를 검증하고 현재 호스트용 실행 파일만 꺼내 실행합니다. 다른 플랫폼의 실행 파일은 교차 빌드하지만 이 스크립트에서 실행하지 않습니다.

## GitHub 게시

공식 저장소는 [jisung-02/Synctest-Scout](https://github.com/jisung-02/Synctest-Scout)이며 모듈 경로는 `github.com/jisung-02/Synctest-Scout`입니다.

```sh
go install github.com/jisung-02/Synctest-Scout/cmd/synctest-scout@latest
```

1. 원하는 커밋에서 **Quality gate** 성공을 확인합니다.
2. 필요하면 저장소 규칙에서 해당 검사를 필수로 지정합니다.
3. 버전을 게시할 준비가 되면 annotated tag를 생성하고 푸시합니다.

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

검사 통과 후 실제로 게시합니다. `v0.1.0-rc.1`은 prerelease로 게시됩니다. 잘못된 버전은 패키징 전에 실패합니다. 저장소의 기본 `GITHUB_TOKEN`을 사용하므로 별도 배포 계정이나 커버리지 서비스 토큰이 필요 없습니다.
