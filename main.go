package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	messageBox                = user32.NewProc("MessageBoxW")
	shell32                   = syscall.NewLazyDLL("shell32.dll")
	shBrowseForFolder         = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDList       = shell32.NewProc("SHGetPathFromIDListW")
	comdlg32                  = syscall.NewLazyDLL("comdlg32.dll")
	getOpenFileName           = comdlg32.NewProc("GetOpenFileNameW")
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	getEnvironmentVariable    = kernel32.NewProc("GetEnvironmentVariableW")
	setCurrentDirectoryProc   = kernel32.NewProc("SetCurrentDirectoryW")
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	openProcessToken          = advapi32.NewProc("OpenProcessToken")
	getTokenInformation       = advapi32.NewProc("GetTokenInformation")
)

// 管理者権限かどうかをチェックする
func isAdmin() bool {
	var token syscall.Token
		
	currentProcess, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}

	// プロセストークンを開く
	err = syscall.OpenProcessToken(currentProcess, syscall.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	// TOKEN_ELEVATION構造体のサイズを取得
	var elevationInfo uint32
	var outLen uint32
	
	// TokenElevation = 20
	err = syscall.GetTokenInformation(
		token, 
		20, // TokenElevation
		(*byte)(unsafe.Pointer(&elevationInfo)), 
		uint32(unsafe.Sizeof(elevationInfo)), 
		&outLen,
	)
	if err != nil {
		return false
	}

	return elevationInfo != 0
}

// 管理者権限で自分自身を再起動する
func runAsAdmin() error {
	verb := syscall.StringToUTF16Ptr("runas")
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exePath := syscall.StringToUTF16Ptr(exe)
	
	// ShellExecute関数を使用して管理者権限で再起動
	shellExecute := shell32.NewProc("ShellExecuteW")
	ret, _, _ := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePath)),
		0,
		0,
		syscall.SW_SHOW,
	)

	if ret <= 32 { // エラーの場合
		return fmt.Errorf("管理者権限での起動に失敗しました")
	}
	
	return nil
}

// Windows API構造体
type BROWSEINFO struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

type OPENFILENAME struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

// メッセージボックスを表示
func showMessageBox(title, message string, flags uint) int {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	ret, _, _ := messageBox.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(flags),
	)
	return int(ret)
}

// ファイル選択ダイアログを表示
func selectFile(title string, filter string) (string, error) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	filterPtr, _ := syscall.UTF16PtrFromString(filter)

	// バッファを用意
	buf := make([]uint16, 260)
	// バッファをヌル文字で初期化
	for i := range buf {
		buf[i] = 0
	}

	ofn := OPENFILENAME{
		lStructSize:     uint32(unsafe.Sizeof(OPENFILENAME{})),
		hwndOwner:       0,
		lpstrFilter:     filterPtr,
		lpstrFile:       &buf[0],
		nMaxFile:        uint32(len(buf)),
		lpstrTitle:      titlePtr,
		flags:           0x00080000 | 0x00001000, // OFN_EXPLORER | OFN_FILEMUSTEXIST
	}

	ret, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))

	if ret == 0 {
		return "", fmt.Errorf("ファイル選択がキャンセルされました")
	}

	return syscall.UTF16ToString(buf), nil
}

// ProgramDataフォルダのパスを取得
func getProgramDataPath() (string, error) {
	buf := make([]uint16, 260)
	n, _, _ := getEnvironmentVariable.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("ProgramData"))),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if n == 0 {
		return "", fmt.Errorf("ProgramDataフォルダのパスを取得できませんでした")
	}
	return syscall.UTF16ToString(buf[:n]), nil
}

// レジストリキーを開く関数
func openRegistryKey(keyPath string) (syscall.Handle, error) {
	var handle syscall.Handle
	err := syscall.RegOpenKeyEx(
		syscall.HKEY_LOCAL_MACHINE, 
		syscall.StringToUTF16Ptr(keyPath), 
		0, 
		syscall.KEY_READ, 
		&handle,
	)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

// レジストリ値を取得する関数
func getRegistryValue(handle syscall.Handle, valueName string) (string, error) {
	var bufSize uint32
	var valueType uint32
	
	// 必要なバッファサイズを取得
	err := syscall.RegQueryValueEx(
		handle,
		syscall.StringToUTF16Ptr(valueName),
		nil,
		&valueType,
		nil,
		&bufSize,
	)
	if err != nil {
		return "", err
	}
	
	// バッファを確保
	buf := make([]uint16, bufSize/2+1)
	
	// 値を取得
	err = syscall.RegQueryValueEx(
		handle,
		syscall.StringToUTF16Ptr(valueName),
		nil,
		&valueType,
		(*byte)(unsafe.Pointer(&buf[0])),
		&bufSize,
	)
	if err != nil {
		return "", err
	}
	
	return syscall.UTF16ToString(buf), nil
}

// OBSインストール先を取得
func getOBSInstallPath() (string, error) {
	// レジストリからOBSインストール先を取得
	handle, err := openRegistryKey(`SOFTWARE\OBS Studio`)
	if err == nil {
		defer syscall.RegCloseKey(handle)
		path, err := getRegistryValue(handle, "Default Install Path")
		if err == nil && path != "" {
			return path, nil
		}
	}

	// 一般的なインストール先を確認
	commonPaths := []string{
		`C:\Program Files\obs-studio`,
		`C:\Program Files (x86)\obs-studio`,
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("OBSのインストール先が見つかりませんでした")
}

// ディレクトリが存在しない場合に作成
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, os.ModePerm)
	}
	return nil
}

// ZIPファイルを解凍
func unzipFile(zipFile, destDir string) error {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// ディレクトリの場合
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// ファイルパスのディレクトリ部分を確認および作成
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// ファイルを解凍
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// ディレクトリ内のファイルを別のディレクトリにコピー
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 相対パスを計算
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// 送信先のパス
		dstPath := filepath.Join(dst, relPath)

		// ディレクトリの場合
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// ファイルの場合
		return copyFile(path, dstPath)
	})
}

// ファイルをコピー
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 送信先のディレクトリを確認および作成
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// DLLファイルを移動
func moveDLLFiles(hookDir string) error {
	// oldフォルダを作成
	oldDir := filepath.Join(hookDir, "old")
	if err := ensureDir(oldDir); err != nil {
		return err
	}

	// DLLファイルを検索して移動
	matches, err := filepath.Glob(filepath.Join(hookDir, "*.dll"))
	if err != nil {
		return err
	}

	for _, file := range matches {
		fileName := filepath.Base(file)
		targetPath := filepath.Join(oldDir, fileName)
		fmt.Printf("%s を %s に移動しています...\n", fileName, oldDir)
		
		// 既存のファイルを削除
		_ = os.Remove(targetPath)
		
		// 移動
		if err := os.Rename(file, targetPath); err != nil {
			fmt.Printf("警告: %s の移動に失敗しました: %v\n", fileName, err)
		}
	}

	return nil
}

func main() {
	fmt.Println("OBS Stduioでゲームがキャプチャ出来ないとき用")
	fmt.Println("-------------------------")
	fmt.Println("問い合わせ先 X: @ureshinovt")

	// 管理者権限のチェック
	if !isAdmin() {
		fmt.Println("管理者権限がありません。管理者として再起動します...")
		err := runAsAdmin()
		if err != nil {
			fmt.Printf("管理者権限での起動に失敗しました: %v\n", err)
			showMessageBox("エラー", "このプログラムは管理者権限で実行する必要があります。\n右クリックして「管理者として実行」を選択してください。", 0x10)
		}
		// 終了して管理者権限の新しいプロセスに譲る
		os.Exit(0)
	}

	fmt.Println("管理者権限で実行中...")

	// ProgramDataフォルダのパスを取得
	programData, err := getProgramDataPath()
	if err != nil {
		showMessageBox("エラー", fmt.Sprintf("ProgramDataフォルダのパスを取得できませんでした: %v", err), 0x10)
		return
	}

	// OBS-Studio-Hookディレクトリのパスを設定
	hookDir := filepath.Join(programData, "obs-studio-hook")
	fmt.Printf("ProgramData\\obs-studio-hook フォルダのフルパス: %s\n", hookDir)

	// OBSインストール先を取得
	obsDir, err := getOBSInstallPath()
	if err != nil {
		result := showMessageBox("エラー", fmt.Sprintf("OBSのインストール先が見つかりません。手動で選択しますか？"), 0x24)
		if result == 6 { // YES
			// この部分はGUIで実装する必要があります
			fmt.Println("現在のバージョンでは手動選択はサポートされていません。")
			return
		} else {
			return
		}
	}

	// win-captureディレクトリのパスを設定
	captureDir := filepath.Join(obsDir, "data", "obs-plugins", "win-capture")
	fmt.Printf("OBS win-capture フォルダのフルパス: %s\n", captureDir)

	// ディレクトリの存在確認
	if _, err := os.Stat(hookDir); os.IsNotExist(err) {
		showMessageBox("エラー", fmt.Sprintf("%s が見つかりません。", hookDir), 0x10)
		return
	}

	if _, err := os.Stat(captureDir); os.IsNotExist(err) {
		showMessageBox("エラー", fmt.Sprintf("%s が見つかりません。", captureDir), 0x10)
		return
	}

	// ZIPファイルを選択
	zipFile, err := selectFile("更新用のZIPファイルを選択してください", "ZIPファイル (*.zip)\x00*.zip\x00すべてのファイル (*.*)\x00*.*\x00")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("選択されたZIPファイル: %s\n", zipFile)

	// DLLファイルを移動
	fmt.Println("\nステップ1: DLLファイルをバックアップ中...")
	if err := moveDLLFiles(hookDir); err != nil {
		showMessageBox("エラー", fmt.Sprintf("DLLファイルの移動中にエラーが発生しました: %v", err), 0x10)
		return
	}

	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "obs_update_*")
	if err != nil {
		showMessageBox("エラー", fmt.Sprintf("一時ディレクトリの作成に失敗しました: %v", err), 0x10)
		return
	}
	defer os.RemoveAll(tempDir)

	// ZIPファイルを解凍
	fmt.Println("\nステップ2: ZIPファイルを展開中...")
	if err := unzipFile(zipFile, tempDir); err != nil {
		showMessageBox("エラー", fmt.Sprintf("ZIPファイルの展開に失敗しました: %v", err), 0x10)
		return
	}

	// win-captureフォルダを検索
	winCapturePath := ""
	standardPath := filepath.Join(tempDir, "data", "obs-plugins", "win-capture")
	if _, err := os.Stat(standardPath); err == nil {
		winCapturePath = standardPath
	} else {
		// 他のパターンを検索
		alternativePath := filepath.Join(tempDir, "win-capture")
		if _, err := os.Stat(alternativePath); err == nil {
			winCapturePath = alternativePath
		} else {
			// 再帰的に win-capture フォルダを探す
			err = filepath.Walk(tempDir, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && filepath.Base(path) == "win-capture" {
					winCapturePath = path
					return filepath.SkipAll
				}
				return nil
			})
			if err != nil {
				showMessageBox("エラー", fmt.Sprintf("ZIPファイル内の検索中にエラーが発生しました: %v", err), 0x10)
				return
			}
		}
	}

	if winCapturePath == "" {
		showMessageBox("エラー", "ZIPファイル内に win-capture フォルダが見つかりませんでした。", 0x10)
		return
	}

	// win-captureフォルダのファイルをコピー
	fmt.Println("\nステップ3: win-capture フォルダの内容をコピー中...")
	if err := copyDir(winCapturePath, captureDir); err != nil {
		showMessageBox("エラー", fmt.Sprintf("ファイルのコピー中にエラーが発生しました: %v", err), 0x10)
		return
	}

	fmt.Println("\n処理が完了しました！")
	fmt.Println("※更新を適用するには OBS Studio を再起動してください。")
	
	showMessageBox("完了", "OBSの更新処理が完了しました。\n\n更新を適用するには OBS Studio を再起動してください。", 0x40)
}