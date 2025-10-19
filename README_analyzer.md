# GitHub Contribution Analyzer

Two powerful scripts to analyze GitHub repository contributions and identify non-maintainer contributors using the GitHub CLI.

## Prerequisites

1. **GitHub CLI**: Install from https://cli.github.com/
2. **Authentication**: Run `gh auth login` to authenticate
3. **Python 3.6+** (for the advanced analyzer)
4. **jq** and **bc** (for the bash analyzer)

## Scripts Overview

### 1. Basic Bash Analyzer (`analyze_github_contributions.sh`)
- Lightweight bash script using `gh` CLI
- Quick analysis with basic metrics
- Good for simple reports

### 2. Advanced Python Analyzer (`advanced_github_analyzer.py`)
- More sophisticated pattern recognition
- Enhanced maintainer identification
- Detailed JSON output and insights

## Usage

### Basic Analyzer
```bash
# Analyze a repository
./analyze_github_contributions.sh microsoft/vscode

# Or with GitHub URL
./analyze_github_contributions.sh https://github.com/microsoft/vscode
```

### Advanced Analyzer
```bash
# Basic usage (30 days by default)
./advanced_github_analyzer.py microsoft/vscode

# Custom time period
./advanced_github_analyzer.py microsoft/vscode --days 60

# Adjust maintainer threshold
./advanced_github_analyzer.py microsoft/vscode --min-maintainer-commits 20

# With GitHub URL
./advanced_github_analyzer.py https://github.com/microsoft/vscode
```

## Features

### Maintainer Identification
- **Commit frequency**: Users with 10+ commits (configurable)
- **Repository access**: Collaborators and team members
- **PR merge activity**: Users with frequent merged PRs

### Analysis Metrics
- Total non-maintainers
- PRs from non-maintainers vs total PRs
- Contribution percentages
- Top 20 contributors ranking
- PR merge rates
- Average contributions per user

### Pattern Recognition
- Merge commit detection
- PR number extraction
- Squash and rebase merge identification
- Author classification

## Output Format

```
GITHUB REPOSITORY CONTRIBUTION ANALYSIS
Repository: microsoft/vscode
Analysis Period: Last 30 days

SUMMARY METRICS:
• Total maintainers:           15
• Total non-maintainers:       45  
• Total PRs from non-maintainers: 67
• Total PRs overall:           120
• Non-maintainer contribution: 55.8%

TOP 20 NON-MAINTAINERS BY PR COUNT:
Rank   Username                  PR Count   Contribution %
────────────────────────────────────────────────────────
1      contributor1              8          6.7%
2      contributor2              5          4.2%
...
```

## Data Export

The advanced analyzer saves detailed JSON data with:
- Complete PR information
- Maintainer lists
- Contribution statistics
- Analysis metadata

## Configuration

### Environment Variables
```bash
export GITHUB_TOKEN="your_token"  # Optional, if not using gh auth
```

### Script Configuration
Edit the scripts to adjust:
- `MIN_MAINTAINER_COMMITS`: Threshold for maintainer classification
- `ANALYSIS_DAYS`: Time period for analysis
- `MAX_RESULTS`: Number of top contributors to display

## Examples

### Popular Repositories
```bash
# Analyze Docker
./advanced_github_analyzer.py moby/moby --days 90

# Analyze React
./analyze_github_contributions.sh facebook/react

# Analyze Kubernetes
./advanced_github_analyzer.py kubernetes/kubernetes --days 14
```

## Troubleshooting

### Common Issues
1. **Authentication Error**: Run `gh auth login`
2. **Repository Not Found**: Check repository name and permissions
3. **API Rate Limits**: Use authenticated requests with GitHub token
4. **Large Repositories**: Increase timeout or reduce analysis period

### Performance Tips
- For large repos, start with shorter time periods (7-14 days)
- The advanced analyzer handles pagination automatically
- Use the basic analyzer for quick checks

## API Limits

GitHub API has rate limits:
- **Authenticated**: 5,000 requests/hour
- **Unauthenticated**: 60 requests/hour

The scripts use authenticated requests via `gh` CLI to maximize limits.