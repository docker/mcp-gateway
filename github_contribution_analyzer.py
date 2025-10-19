#!/usr/bin/env python3
"""
Simplified GitHub Repository Contribution Analysis
Working version using GitHub CLI
"""

import json
import subprocess
import sys
import re
from datetime import datetime, timedelta
from collections import defaultdict, Counter
import argparse

def run_gh_command(cmd):
    """Run a GitHub CLI command and return output"""
    try:
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        return result.stdout.strip()
    except subprocess.CalledProcessError as e:
        print(f"Error: {e.stderr}")
        return ""

def fetch_commits(repo_full, days=30):
    """Fetch recent commits"""
    print(f"🔍 Fetching commits for {repo_full}...")
    
    # Get commits from last N days - use basic API without date filter for now
    cmd = ["gh", "api", f"repos/{repo_full}/commits", "--paginate", "--jq", ".[].sha"]
    commit_shas = run_gh_command(cmd)
    
    if not commit_shas:
        return []
    
    # Get detailed commit info
    commits = []
    cutoff_date = datetime.now() - timedelta(days=days)
    
    for sha in commit_shas.split('\n')[:200]:  # Limit for testing
        sha = sha.strip().strip('"')
        if not sha:
            continue
            
        cmd = ["gh", "api", f"repos/{repo_full}/commits/{sha}"]
        commit_data = run_gh_command(cmd)
        if commit_data:
            try:
                commit = json.loads(commit_data)
                date_str = commit.get('commit', {}).get('author', {}).get('date', '')
                if date_str:
                    commit_date = datetime.fromisoformat(date_str.replace('Z', '+00:00')).replace(tzinfo=None)
                    if commit_date >= cutoff_date:
                        commits.append(commit)
            except:
                continue
    
    print(f"📊 Found {len(commits)} commits from last {days} days")
    return commits

def fetch_pull_requests(repo_full, days=30):
    """Fetch recent pull requests"""
    print(f"🔍 Fetching pull requests for {repo_full}...")
    
    cmd = ["gh", "pr", "list", "--repo", repo_full, "--state", "all", "--limit", "200", "--json", "number,author,title,createdAt,mergedAt,state"]
    output = run_gh_command(cmd)
    
    if not output:
        return []
    
    try:
        all_prs = json.loads(output)
        cutoff_date = datetime.now() - timedelta(days=days)
        recent_prs = []
        
        for pr in all_prs:
            created_at_str = pr.get('createdAt', '')
            if created_at_str:
                created_at = datetime.fromisoformat(created_at_str.replace('Z', '+00:00')).replace(tzinfo=None)
                if created_at >= cutoff_date:
                    recent_prs.append(pr)
        
        print(f"📊 Found {len(recent_prs)} PRs from last {days} days")
        return recent_prs
    except json.JSONDecodeError:
        print("❌ Failed to parse PR data")
        return []

def analyze_contributions(commits, prs, min_maintainer_commits=10):
    """Analyze contributions and identify maintainers vs contributors"""
    
    # Count commits by author
    commit_authors = defaultdict(int)
    merge_commits = []
    
    for commit in commits:
        author_info = commit.get('author') or {}
        author = author_info.get('login') or commit.get('commit', {}).get('author', {}).get('name', 'Unknown')
        message = commit.get('commit', {}).get('message', '')
        
        commit_authors[author] += 1
        
        # Detect merge commits
        if re.search(r'Merge pull request #(\d+)|Merge branch|^Merge', message, re.IGNORECASE):
            pr_match = re.search(r'#(\d+)', message)
            merge_commits.append({
                'author': author,
                'message': message,
                'pr_number': pr_match.group(1) if pr_match else None
            })
    
    # Count PRs by author
    pr_authors = defaultdict(int)
    for pr in prs:
        author = pr.get('author', {}).get('login', 'Unknown')
        pr_authors[author] += 1
    
    # Identify maintainers (high commit count or high PR count)
    maintainers = set()
    for author, count in commit_authors.items():
        if count >= min_maintainer_commits:
            maintainers.add(author)
    
    for author, count in pr_authors.items():
        if count >= 5:  # High PR count threshold
            maintainers.add(author)
    
    # Calculate non-maintainer statistics
    non_maintainer_prs = []
    for pr in prs:
        author = pr.get('author', {}).get('login', 'Unknown')
        if author not in maintainers and author != 'Unknown':
            non_maintainer_prs.append(pr)
    
    non_maintainer_counts = Counter(pr.get('author', {}).get('login', 'Unknown') for pr in non_maintainer_prs)
    
    return {
        'maintainers': maintainers,
        'commit_authors': dict(commit_authors),
        'pr_authors': dict(pr_authors),
        'non_maintainer_prs': non_maintainer_prs,
        'non_maintainer_counts': dict(non_maintainer_counts),
        'merge_commits': merge_commits
    }

def generate_report(repo_full, analysis, days):
    """Generate and print the analysis report"""
    maintainers = analysis['maintainers']
    non_maintainer_counts = analysis['non_maintainer_counts']
    non_maintainer_prs = analysis['non_maintainer_prs']
    total_prs = len(analysis['pr_authors'])
    
    print("\n" + "="*80)
    print("           GITHUB REPOSITORY CONTRIBUTION ANALYSIS")
    print("="*80)
    print(f"\n📂 Repository: {repo_full}")
    print(f"📅 Analysis Period: Last {days} days")
    print(f"🕒 Analysis Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    
    print(f"\n📊 SUMMARY METRICS:")
    print("-" * 50)
    print(f"• Total maintainers:           {len(maintainers)}")
    print(f"• Total non-maintainers:       {len(non_maintainer_counts)}")
    print(f"• Total PRs from non-maintainers: {len(non_maintainer_prs)}")
    print(f"• Total PRs overall:           {total_prs}")
    
    if total_prs > 0:
        percentage = (len(non_maintainer_prs) / total_prs) * 100
        print(f"• Non-maintainer contribution: {percentage:.1f}%")
    
    print(f"\n🏆 TOP 20 NON-MAINTAINERS BY PR COUNT:")
    print("-" * 70)
    print(f"{'Rank':<6} {'Username':<25} {'PR Count':<10} {'Contribution %':<15}")
    print("-" * 70)
    
    sorted_contributors = sorted(non_maintainer_counts.items(), key=lambda x: x[1], reverse=True)
    
    for rank, (username, count) in enumerate(sorted_contributors[:20], 1):
        percentage = (count / total_prs * 100) if total_prs > 0 else 0
        print(f"{rank:<6} {username:<25} {count:<10} {percentage:<15.1f}%")
    
    print(f"\n👥 IDENTIFIED MAINTAINERS:")
    print("-" * 40)
    maintainer_list = sorted(list(maintainers))
    for i, maintainer in enumerate(maintainer_list[:15], 1):
        commits = analysis['commit_authors'].get(maintainer, 0)
        prs = analysis['pr_authors'].get(maintainer, 0)
        print(f"{i:>3}. {maintainer} ({commits} commits, {prs} PRs)")
    
    if len(maintainer_list) > 15:
        print(f"    ... and {len(maintainer_list) - 15} more")
    
    print("\n" + "="*80)

def main():
    parser = argparse.ArgumentParser(description='Analyze GitHub repository contributions')
    parser.add_argument('repository', help='Repository in format owner/repo or GitHub URL')
    parser.add_argument('--days', '-d', type=int, default=30, help='Days to analyze (default: 30)')
    
    args = parser.parse_args()
    
    # Parse repository input
    repo_input = args.repository
    if repo_input.startswith('https://github.com/'):
        parts = repo_input.replace('https://github.com/', '').strip('/').split('/')
        repo_full = f"{parts[0]}/{parts[1].replace('.git', '')}"
    elif '/' in repo_input:
        repo_full = repo_input
    else:
        print("❌ Invalid repository format. Use 'owner/repo' or GitHub URL.")
        sys.exit(1)
    
    # Check GitHub CLI authentication
    try:
        subprocess.run(["gh", "auth", "status"], check=True, capture_output=True)
    except (subprocess.CalledProcessError, FileNotFoundError):
        print("❌ GitHub CLI not authenticated. Please run 'gh auth login' first.")
        sys.exit(1)
    
    print(f"🚀 Starting analysis for {repo_full}...")
    
    # Fetch data
    commits = fetch_commits(repo_full, args.days)
    prs = fetch_pull_requests(repo_full, args.days)
    
    if not commits and not prs:
        print("❌ No data found. Repository may be private or doesn't exist.")
        sys.exit(1)
    
    # Analyze
    analysis = analyze_contributions(commits, prs)
    
    # Generate report
    generate_report(repo_full, analysis, args.days)
    
    # Save data
    filename = f"github_analysis_{repo_full.replace('/', '_')}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump({
            'repository': repo_full,
            'analysis_date': datetime.now().isoformat(),
            'period_days': args.days,
            'analysis': {k: list(v) if isinstance(v, set) else v for k, v in analysis.items()}
        }, f, indent=2, default=str)
    
    print(f"\n💾 Detailed data saved to: {filename}")

if __name__ == '__main__':
    main()