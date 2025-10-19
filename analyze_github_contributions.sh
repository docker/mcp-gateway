#!/bin/bash

# GitHub Repository Contribution Analysis Script
# Analyzes non-maintainer contributions using gh CLI

set -e

# Configuration
ANALYSIS_DAYS=30
MIN_MAINTAINER_COMMITS=10  # Minimum commits to be considered a maintainer
MAX_RESULTS=20

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_header() {
    echo -e "${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

print_error() {
    echo -e "${RED}$1${NC}"
}

# Function to check if gh CLI is installed and authenticated
check_gh_auth() {
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed. Please install it first."
        exit 1
    fi
    
    if ! gh auth status &> /dev/null; then
        print_error "GitHub CLI is not authenticated. Please run 'gh auth login' first."
        exit 1
    fi
}

# Function to extract repository info from URL or validate owner/repo format
parse_repo_input() {
    local input="$1"
    
    if [[ $input =~ ^https://github\.com/([^/]+)/([^/]+) ]]; then
        # Extract from GitHub URL
        REPO_OWNER="${BASH_REMATCH[1]}"
        REPO_NAME="${BASH_REMATCH[2]}"
        # Remove .git suffix if present
        REPO_NAME="${REPO_NAME%.git}"
    elif [[ $input =~ ^([^/]+)/([^/]+)$ ]]; then
        # Direct owner/repo format
        REPO_OWNER="${BASH_REMATCH[1]}"
        REPO_NAME="${BASH_REMATCH[2]}"
    else
        print_error "Invalid repository format. Use 'owner/repo' or GitHub URL."
        exit 1
    fi
}

# Function to get commits from the last N days
get_recent_commits() {
    local since_date=$(date -d "${ANALYSIS_DAYS} days ago" +%Y-%m-%d 2>/dev/null || date -v-${ANALYSIS_DAYS}d +%Y-%m-%d)
    
    print_warning "Fetching commits since $since_date..."
    
    # Get commits with author, message, and SHA
    gh api "repos/${REPO_OWNER}/${REPO_NAME}/commits" \
        --paginate \
        -f since="${since_date}T00:00:00Z" \
        --jq '.[] | {
            sha: .sha[0:7],
            author: .author.login // .commit.author.name,
            message: .commit.message,
            date: .commit.author.date
        }' > commits_raw.json
}

# Function to analyze commits and extract PR information
analyze_commits() {
    print_warning "Analyzing commit patterns..."
    
    # Create temporary files for analysis
    > maintainers.txt
    > contributors.txt
    > prs.json
    > author_stats.json
    
    # Process commits to identify patterns
    jq -r '.author' commits_raw.json | sort | uniq -c | sort -rn > author_commit_counts.txt
    
    # Identify maintainers (users with many commits)
    awk -v min_commits="$MIN_MAINTAINER_COMMITS" '$1 >= min_commits {print $2}' author_commit_counts.txt > maintainers.txt
    
    # Extract PR information from merge commits
    jq -r 'select(.message | test("Merge pull request #[0-9]+|Merge branch|^Merge")) | 
        {
            pr_number: (.message | capture("Merge pull request #(?<num>[0-9]+)"; "g").num // "unknown"),
            author: .author,
            message: .message,
            date: .date
        }' commits_raw.json > prs.json
    
    # Get PR details for better analysis
    print_warning "Fetching PR details..."
    > pr_details.json
    
    while IFS= read -r pr_number; do
        if [[ "$pr_number" != "unknown" && "$pr_number" != "null" ]]; then
            if pr_data=$(gh api "repos/${REPO_OWNER}/${REPO_NAME}/pulls/${pr_number}" 2>/dev/null); then
                echo "$pr_data" | jq '{
                    number: .number,
                    author: .user.login,
                    title: .title,
                    merged_at: .merged_at,
                    state: .state
                }' >> pr_details.json
            fi
        fi
    done < <(jq -r '.pr_number' prs.json | grep -v null | sort -u)
}

# Function to classify contributors and generate statistics
generate_analysis() {
    print_warning "Generating contribution analysis..."
    
    # Count total PRs
    TOTAL_PRS=$(jq -s 'length' pr_details.json)
    
    # Create comprehensive author statistics
    jq -s '
        group_by(.author) | 
        map({
            author: .[0].author,
            pr_count: length,
            prs: map(.number)
        }) | 
        sort_by(-.pr_count)
    ' pr_details.json > author_pr_stats.json
    
    # Identify non-maintainers
    > non_maintainer_stats.json
    while IFS= read -r author_data; do
        author=$(echo "$author_data" | jq -r '.author')
        if ! grep -q "^${author}$" maintainers.txt; then
            echo "$author_data" >> non_maintainer_stats.json
        fi
    done < <(jq -c '.[]' author_pr_stats.json)
    
    # Calculate statistics
    NON_MAINTAINER_COUNT=$(jq -s 'length' non_maintainer_stats.json)
    NON_MAINTAINER_PRS=$(jq -s 'map(.pr_count) | add // 0' non_maintainer_stats.json)
    MAINTAINER_COUNT=$(wc -l < maintainers.txt)
}

# Function to display the final report
display_report() {
    clear
    print_header "════════════════════════════════════════════════════════════════"
    print_header "           GITHUB REPOSITORY CONTRIBUTION ANALYSIS"
    print_header "════════════════════════════════════════════════════════════════"
    echo ""
    
    print_success "Repository: ${REPO_OWNER}/${REPO_NAME}"
    print_success "Analysis Period: Last ${ANALYSIS_DAYS} days"
    print_success "Analysis Date: $(date)"
    echo ""
    
    print_header "SUMMARY METRICS:"
    print_header "────────────────────────────────────────────────────────────────"
    printf "%-30s %s\n" "• Total maintainers:" "$MAINTAINER_COUNT"
    printf "%-30s %s\n" "• Total non-maintainers:" "$NON_MAINTAINER_COUNT"
    printf "%-30s %s\n" "• Total PRs from non-maintainers:" "$NON_MAINTAINER_PRS"
    printf "%-30s %s\n" "• Total PRs overall:" "$TOTAL_PRS"
    
    if [[ $TOTAL_PRS -gt 0 ]]; then
        NON_MAINTAINER_PERCENTAGE=$(echo "scale=1; $NON_MAINTAINER_PRS * 100 / $TOTAL_PRS" | bc -l 2>/dev/null || echo "0.0")
        printf "%-30s %s%%\n" "• Non-maintainer contribution:" "$NON_MAINTAINER_PERCENTAGE"
    fi
    echo ""
    
    print_header "TOP $MAX_RESULTS NON-MAINTAINERS BY PR COUNT:"
    print_header "────────────────────────────────────────────────────────────────"
    printf "%-6s %-25s %-10s %-15s\n" "Rank" "Username" "PR Count" "Contribution %"
    print_header "────────────────────────────────────────────────────────────────"
    
    # Display top non-maintainers
    jq -r '.[] | "\(.author) \(.pr_count)"' non_maintainer_stats.json | head -n $MAX_RESULTS | nl | while read -r rank username pr_count; do
        if [[ $TOTAL_PRS -gt 0 ]]; then
            percentage=$(echo "scale=1; $pr_count * 100 / $TOTAL_PRS" | bc -l 2>/dev/null || echo "0.0")
        else
            percentage="0.0"
        fi
        printf "%-6s %-25s %-10s %-15s\n" "$rank" "$username" "$pr_count" "${percentage}%"
    done
    
    echo ""
    print_header "IDENTIFIED MAINTAINERS (${MIN_MAINTAINER_COMMITS}+ commits):"
    print_header "────────────────────────────────────────────────────────────────"
    if [[ -s maintainers.txt ]]; then
        cat maintainers.txt | head -10 | nl -w3 -s". "
        if [[ $(wc -l < maintainers.txt) -gt 10 ]]; then
            echo "... and $(($(wc -l < maintainers.txt) - 10)) more"
        fi
    else
        echo "No maintainers identified with current criteria"
    fi
    
    echo ""
    print_success "Analysis complete! Data files saved in current directory."
}

# Function to cleanup temporary files
cleanup() {
    rm -f commits_raw.json prs.json pr_details.json
    rm -f author_commit_counts.txt maintainers.txt contributors.txt
    rm -f author_pr_stats.json non_maintainer_stats.json
}

# Main execution
main() {
    if [[ $# -ne 1 ]]; then
        echo "Usage: $0 <owner/repo or GitHub URL>"
        echo "Example: $0 microsoft/vscode"
        echo "Example: $0 https://github.com/microsoft/vscode"
        exit 1
    fi
    
    check_gh_auth
    parse_repo_input "$1"
    
    print_header "Starting analysis for ${REPO_OWNER}/${REPO_NAME}..."
    
    # Check if repository exists
    if ! gh repo view "${REPO_OWNER}/${REPO_NAME}" &>/dev/null; then
        print_error "Repository ${REPO_OWNER}/${REPO_NAME} not found or not accessible."
        exit 1
    fi
    
    get_recent_commits
    analyze_commits
    generate_analysis
    display_report
    
    # Ask if user wants to keep data files
    echo ""
    read -p "Keep analysis data files? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        cleanup
    fi
}

# Run main function with all arguments
main "$@"