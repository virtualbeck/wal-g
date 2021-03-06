package internal

import (
	"github.com/pkg/errors"
	"github.com/wal-g/tracelog"
	"os"
	"path/filepath"
)

// temporary flag is used in tar interpreter to determine if it should use new unwrap logic
var useNewUnwrapImplementation = false

func checkDbDirectoryForUnwrapNew(dbDataDirectory string, sentinelDto BackupSentinelDto) error {
	tracelog.DebugLogger.Println("DB data directory before applying backup:")
	_ = filepath.Walk(dbDataDirectory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				tracelog.DebugLogger.Println(path)
			}
			return nil
		})

	for fileName, fileDescription := range sentinelDto.Files {
		if fileDescription.IsSkipped {
			tracelog.DebugLogger.Printf("Skipped file %v\n", fileName)
		}
	}

	if sentinelDto.TablespaceSpec != nil && !sentinelDto.TablespaceSpec.empty() {
		err := setTablespacePaths(*sentinelDto.TablespaceSpec)
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO : unit tests
// Do the job of unpacking Backup object
func (backup *Backup) unwrapNew(
	dbDataDirectory string, sentinelDto BackupSentinelDto, filesToUnwrap map[string]bool, createIncrementalFiles bool,
) error {
	useNewUnwrapImplementation = true
	err := checkDbDirectoryForUnwrapNew(dbDataDirectory, sentinelDto)
	if err != nil {
		return err
	}

	tarInterpreter := NewFileTarInterpreter(dbDataDirectory, sentinelDto, filesToUnwrap, createIncrementalFiles)
	tarsToExtract, pgControlKey, err := backup.getTarsToExtract(sentinelDto, filesToUnwrap)
	if err != nil {
		return err
	}

	// Check name for backwards compatibility. Will check for `pg_control` if WALG version of backup.
	needPgControl := IsPgControlRequired(backup, sentinelDto)

	if pgControlKey == "" && needPgControl {
		return newPgControlNotFoundError()
	}

	err = ExtractAll(tarInterpreter, tarsToExtract)
	if err != nil {
		return err
	}

	if needPgControl {
		err = ExtractAll(tarInterpreter, []ReaderMaker{newStorageReaderMaker(backup.getTarPartitionFolder(), pgControlKey)})
		if err != nil {
			return errors.Wrap(err, "failed to extract pg_control")
		}
	}

	tracelog.InfoLogger.Print("\nBackup extraction complete.\n")
	return nil
}
