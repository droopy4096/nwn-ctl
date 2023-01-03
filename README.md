# Neverwinter Nights module management utility

## Options

```
  -backup-extension string
         (default ".bak")
  -command string
         (default "install")
  -dry-run
        Dry Run
  -excluded value
        excluded files
  -extensions-dir string
         (default "Extensions")
  -module string
    
  -nwn-dir string
         (default ".")
  -overwrite-existing
        Overwrite Existing files
  -skip-errors
        Skip file errors
  -target-dir string
```

## Usage

assuming that NWN was installed into `/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond/` under which sub-folder `Extensions` exist that contains unzipped module `foo` (inside directory with the same name):

```
/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond/
 \_ Extensions 
     \_ foo
         \_ ...
```

we can install `foo`:

```shell
nwn-cli -nwn-dir "/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond/" -extensions-dir Extensions -module foo -skip-errors
```

which will result in creation of `foo.json` outlining details of install:


```
/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond/
 \_ Extensions 
     \_ foo
         \_ ...
     \_ foo.json
```

`foo.json`:

```json
{
  "name": "foo",
  "extensionsDir": "Extensions",
  "nwnDir": "/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond",
  "backupExtension": ".bak",
  "overwriteExisting": false,
  "files": [
   "hak/foo1.hak"
  ],
  "installed": [
   "hak/foo1.hak"
  ],
  "skipped": [],
  "excluded": null,
  "saved": null
}
```

which sufficiently outlines install to be used in reverse to remove module:

```shell
nwn-cli -nwn-dir="/home/user1/.wineprefixes/nwn/drive_c/GOG Games/NWN Diamond/" -extensions-dir=Extensions -command=uninstall -module foo
```

which will instruct `nwn-cli` to read `foo.json` manifest and **will replace** `name`, `extensionsDir` and `nwnDir` with values provided in CLI and remove all the installed (declared to be installed) files and rename `foo.json` to `foo.uninstalled` preserving install information for later.
