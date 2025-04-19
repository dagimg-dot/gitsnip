# gitsnip

> A CLI tool to download specific folders from a git repository.

![showcase](./assets/gitsnip-showcase.gif)

[![GitHub release](https://img.shields.io/github/v/release/dagimg-dot/gitsnip)](https://github.com/dagimg-dot/gitsnip/releases/latest)
[![License](https://img.shields.io/github/license/dagimg-dot/gitsnip)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/dagimg-dot/gitsnip/total)](https://github.com/dagimg-dot/gitsnip/releases)

## Features

- 📂 Download specific folders from any Git repository
- 🚀 Fast downloads using sparse checkout or API methods
- 🔒 Support for private repositories
- 🔧 Multiple download methods (API/sparse checkout)
- 🔄 Branch selection support

## Installation

### Using [eget](https://github.com/zyedidia/eget)

```bash
eget dagimg-dot/gitsnip
```

### Manual Installation

#### Linux/macOS

1. Download the appropriate binary for your platform from the [Releases page](https://github.com/dagimg-dot/gitsnip/releases).

2. Extract the binary:
```bash
tar -xzf gitsnip_<os>_<arch>.tar.gz
```

3. Move the binary to a directory in your PATH:
```bash
# Option 1: Move to user's local bin (recommended)
mv gitsnip $HOME/.local/bin/

# Option 2: Move to system-wide bin (requires sudo)
sudo mv gitsnip /usr/local/bin/
```

4. Verify installation by opening a new terminal:
```bash
gitsnip version
```

> Note: For Option 1, make sure `$HOME/.local/bin` is in your PATH. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell's config file (.bashrc, .zshrc, etc.) if needed.

#### Windows

1. Download the Windows binary (`gitsnip_windows_amd64.zip`) from the [Releases page](https://github.com/dagimg-dot/gitsnip/releases).

2. Extract the ZIP file using File Explorer or PowerShell:
```powershell
Expand-Archive -Path gitsnip_windows_amd64.zip -DestinationPath C:\Program Files\gitsnip
```

3. Add to PATH (Choose one method):
   - **Using System Properties:**
     1. Open System Properties (Win + R, type `sysdm.cpl`)
     2. Go to "Advanced" tab → "Environment Variables"
     3. Under "System variables", find and select "Path"
     4. Click "Edit" → "New"
     5. Add `C:\Program Files\gitsnip`

   - **Using PowerShell (requires admin):**
```powershell
$oldPath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
$newPath = $oldPath + ';C:\Program Files\gitsnip'
[Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
```

4. Verify installation by opening a new terminal:
```powershell
gitsnip version
```

## Usage

Basic usage:

```bash
gitsnip <repo-url> <subdir> <output-dir>
```

### Command Options

```bash
Usage:
  gitsnip <repository_url> <folder_path> [output_dir] [flags]
  gitsnip [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Print the version information

Flags:
  -b, --branch string     Repository branch to download from (default "main")
  -h, --help              help for gitsnip
  -m, --method string     Download method ('api' or 'sparse') (default "sparse")
  -p, --provider string   Repository provider ('github', more to come)
  -q, --quiet            Suppress progress output during download
  -t, --token string     GitHub API token for private repositories or increased rate limits
```

### Examples

1. Download a specific folder from a public repository (default method is sparse checkout):

```bash
gitsnip https://github.com/user/repo src/components ./my-components
```

2. Download a specific folder from a public repository using the API method:

```bash
gitsnip https://github.com/user/repo src/components ./my-components -m api
```

3. Download from a specific branch:

```bash
gitsnip https://github.com/user/repo docs ./docs -b develop
```

4. Download from a private repository:

```bash
gitsnip https://github.com/user/private-repo config ./config -t YOUR_GITHUB_TOKEN
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Troubleshooting

### Common Issues

1. **Rate Limit Exceeded**: When using the API method, you might hit GitHub's rate limits. Use a GitHub token to increase the limit or use the sparse checkout method. (See [Usage](#usage))
2. **Permission Denied**: Make sure you have the correct permissions and token for private repositories.

