#!/usr/bin/env python3
"""
Advanced GitHub Repository Contribution Analysis
Enhanced version with better pattern recognition and statistics
"""

import json
import subprocess
import sys
import re
from datetime import datetime, timedelta
from collections import defaultdict, Counter
from typing import Dict, List, Set, Tuple
import argparse

class GitHubAnalyzer:
    def __init__(self, repo_owner: str, repo_name: str, days: int = 30):
        self.repo_owner = repo_owner
        self.repo_name = repo_name
        self.days = days
        self.repo_full = f"{repo_owner}/{repo_name}"
        
        # Analysis data
        self.commits = []
        self.pull_requests = []
        self.maintainers = set()
        self.contributors = {}
        
        # Configuration
        self.min_maintainer_commits = 10
        self.min_maintainer_prs = 5
        
    def check_gh_auth(self) -> bool:
        """Check if GitHub CLI is installed and authenticated"""
        try:
            subprocess.run(["gh", "auth", "status"], 
                         check=True, capture_output=True, text=True)
            return True
        except (subprocess.CalledProcessError, FileNotFoundError):
            return False
    
    def run_gh_command(self, cmd: List[str]) -> str:
        """Run a GitHub CLI command and return output"""
        try:
            result = subprocess.run(cmd, check=True, capture_output=True, text=True)
            return result.stdout
        except subprocess.CalledProcessError as e:
            print(f"Error running command {' '.join(cmd)}: {e}")
            return ""
    
    def fetch_commits(self):
        """Fetch commits from the last N days"""
        since_date = (datetime.now() - timedelta(days=self.days)).strftime('%Y-%m-%dT%H:%M:%SZ')
        
        print(f"🔍 Fetching commits since {since_date[:10]}...")
        
        # Get commits using --paginate for simplicity
        cmd = [
            "gh", "api", f"repos/{self.repo_full}/commits?since={since_date}",
            "--paginate"
        ]
        
        output = self.run_gh_command(cmd)
        if output.strip():
            self.commits = json.loads(output)
        else:
            self.commits = []
        
        print(f"📊 Fetched {len(self.commits)} commits")
    
    def fetch_pull_requests(self):
        """Fetch pull requests from the last N days"""
        print("🔍 Fetching pull requests...")
        
        # Get recent PRs (GitHub API doesn't support since for PRs, so get all recent ones)
        cmd = [
            "gh", "api", f"repos/{self.repo_full}/pulls?state=all&per_page=100",
            "--paginate"
        ]
        
        output = self.run_gh_command(cmd)
        if output.strip():
            all_prs = json.loads(output)
            # Filter by date manually since GitHub API doesn't support since for PRs
            cutoff_date = datetime.now() - timedelta(days=self.days)
            self.pull_requests = []
            for pr in all_prs:
                created_at = datetime.fromisoformat(pr.get('created_at', '').replace('Z', '+00:00'))
                if created_at >= cutoff_date:
                    self.pull_requests.append(pr)
        else:
            self.pull_requests = []
        
        print(f"📊 Fetched {len(self.pull_requests)} pull requests")
    
    def analyze_commit_patterns(self):
        """Analyze commit patterns to identify merge commits and authors"""
        merge_patterns = [
            r"Merge pull request #(\d+)",
            r"Merge branch",
            r"^Merge",
            r"Squash and merge",
            r"Rebase and merge"
        ]
        
        author_commits = defaultdict(int)
        merge_commits = []
        
        for commit in self.commits:
            # Handle potential None values
            author_info = commit.get('author') or {}
            author = author_info.get('login') or commit.get('commit', {}).get('author', {}).get('name', 'Unknown')
            message = commit.get('commit', {}).get('message', '')
            
            author_commits[author] += 1
            
            # Check if it's a merge commit
            for pattern in merge_patterns:
                if re.search(pattern, message, re.IGNORECASE):
                    pr_match = re.search(r"#(\d+)", message)
                    pr_number = pr_match.group(1) if pr_match else None
                    
                    merge_commits.append({
                        'sha': commit.get('sha', '')[:7],
                        'author': author,
                        'message': message,
                        'pr_number': pr_number,
                        'date': commit.get('commit', {}).get('author', {}).get('date', '')
                    })
                    break
        
        self.author_commits = dict(author_commits)
        self.merge_commits = merge_commits
        
        print(f"📈 Found {len(merge_commits)} merge commits from {len(author_commits)} unique authors")
    
    def identify_maintainers(self):
        """Identify maintainers based on commit frequency and PR activity"""
        # Method 1: High commit count
        high_commit_users = {
            author for author, count in self.author_commits.items() 
            if count >= self.min_maintainer_commits
        }
        
        # Method 2: Repository collaborators (if accessible)
        try:
            cmd = ["gh", "api", f"repos/{self.repo_full}/collaborators"]
            output = self.run_gh_command(cmd)
            if output.strip():
                collaborators = {user['login'] for user in json.loads(output)}
                high_commit_users.update(collaborators)
        except:
            pass
        
        # Method 3: Users with many merged PRs
        pr_authors = defaultdict(int)
        for pr in self.pull_requests:
            if pr.get('merged_at'):
                author = pr.get('user', {}).get('login', '')
                if author:
                    pr_authors[author] += 1
        
        high_pr_users = {
            author for author, count in pr_authors.items() 
            if count >= self.min_maintainer_prs
        }
        
        self.maintainers = high_commit_users.union(high_pr_users)
        
        print(f"👥 Identified {len(self.maintainers)} maintainers")
        print(f"   - {len(high_commit_users)} by commit count")
        print(f"   - {len(high_pr_users)} by PR count")
    
    def analyze_contributions(self):
        """Analyze contributions from non-maintainers"""
        non_maintainer_prs = []
        
        for pr in self.pull_requests:
            author = pr.get('user', {}).get('login', '')
            if author and author not in self.maintainers:
                non_maintainer_prs.append({
                    'number': pr.get('number'),
                    'author': author,
                    'title': pr.get('title', ''),
                    'state': pr.get('state', ''),
                    'merged_at': pr.get('merged_at'),
                    'created_at': pr.get('created_at'),
                    'url': pr.get('html_url', '')
                })
        
        # Count contributions by author
        contribution_counts = Counter(pr['author'] for pr in non_maintainer_prs)
        
        self.non_maintainer_prs = non_maintainer_prs
        self.contribution_stats = dict(contribution_counts)
        
        print(f"🎯 Found {len(non_maintainer_prs)} PRs from {len(contribution_counts)} non-maintainers")
    
    def generate_report(self):
        """Generate comprehensive analysis report"""
        total_prs = len(self.pull_requests)
        non_maintainer_pr_count = len(self.non_maintainer_prs)
        non_maintainer_count = len(self.contribution_stats)
        
        print("\n" + "="*80)
        print("           GITHUB REPOSITORY CONTRIBUTION ANALYSIS")
        print("="*80)
        print(f"\n📂 Repository: {self.repo_full}")
        print(f"📅 Analysis Period: Last {self.days} days")
        print(f"🕒 Analysis Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        
        print(f"\n📊 SUMMARY METRICS:")
        print("-" * 50)
        print(f"• Total maintainers:           {len(self.maintainers)}")
        print(f"• Total non-maintainers:       {non_maintainer_count}")
        print(f"• Total PRs from non-maintainers: {non_maintainer_pr_count}")
        print(f"• Total PRs overall:           {total_prs}")
        
        if total_prs > 0:
            percentage = (non_maintainer_pr_count / total_prs) * 100
            print(f"• Non-maintainer contribution: {percentage:.1f}%")
        
        print(f"\n🏆 TOP 20 NON-MAINTAINERS BY PR COUNT:")
        print("-" * 70)
        print(f"{'Rank':<6} {'Username':<25} {'PR Count':<10} {'Contribution %':<15}")
        print("-" * 70)
        
        sorted_contributors = sorted(
            self.contribution_stats.items(), 
            key=lambda x: x[1], 
            reverse=True
        )
        
        for rank, (username, count) in enumerate(sorted_contributors[:20], 1):
            percentage = (count / total_prs * 100) if total_prs > 0 else 0
            print(f"{rank:<6} {username:<25} {count:<10} {percentage:<15.1f}%")
        
        print(f"\n👥 IDENTIFIED MAINTAINERS:")
        print("-" * 40)
        maintainer_list = sorted(list(self.maintainers))
        for i, maintainer in enumerate(maintainer_list[:10], 1):
            commits = self.author_commits.get(maintainer, 0)
            print(f"{i:>3}. {maintainer} ({commits} commits)")
        
        if len(maintainer_list) > 10:
            print(f"    ... and {len(maintainer_list) - 10} more")
        
        # Additional insights
        print(f"\n💡 INSIGHTS:")
        print("-" * 20)
        if non_maintainer_count > 0:
            avg_prs_per_contributor = non_maintainer_pr_count / non_maintainer_count
            print(f"• Average PRs per non-maintainer: {avg_prs_per_contributor:.1f}")
        
        merged_prs = sum(1 for pr in self.non_maintainer_prs if pr['merged_at'])
        if non_maintainer_pr_count > 0:
            merge_rate = (merged_prs / non_maintainer_pr_count) * 100
            print(f"• Non-maintainer PR merge rate: {merge_rate:.1f}%")
        
        print("\n" + "="*80)
    
    def save_detailed_data(self):
        """Save detailed analysis data to JSON files"""
        data = {
            'repository': self.repo_full,
            'analysis_date': datetime.now().isoformat(),
            'period_days': self.days,
            'maintainers': list(self.maintainers),
            'non_maintainer_prs': self.non_maintainer_prs,
            'contribution_stats': self.contribution_stats,
            'total_commits': len(self.commits),
            'total_prs': len(self.pull_requests)
        }
        
        filename = f"github_analysis_{self.repo_owner}_{self.repo_name}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
        with open(filename, 'w') as f:
            json.dump(data, f, indent=2)
        
        print(f"\n💾 Detailed data saved to: {filename}")
    
    def run_analysis(self):
        """Run the complete analysis"""
        if not self.check_gh_auth():
            print("❌ GitHub CLI not authenticated. Please run 'gh auth login' first.")
            return False
        
        print(f"🚀 Starting analysis for {self.repo_full}...")
        
        self.fetch_commits()
        self.fetch_pull_requests()
        self.analyze_commit_patterns()
        self.identify_maintainers()
        self.analyze_contributions()
        self.generate_report()
        self.save_detailed_data()
        
        return True

def parse_repo_input(repo_input: str) -> Tuple[str, str]:
    """Parse repository input (URL or owner/repo format)"""
    if repo_input.startswith('https://github.com/'):
        parts = repo_input.replace('https://github.com/', '').strip('/').split('/')
        return parts[0], parts[1].replace('.git', '')
    elif '/' in repo_input:
        parts = repo_input.split('/')
        return parts[0], parts[1]
    else:
        raise ValueError("Invalid repository format")

def main():
    parser = argparse.ArgumentParser(description='Analyze GitHub repository contributions')
    parser.add_argument('repository', help='Repository in format owner/repo or GitHub URL')
    parser.add_argument('--days', '-d', type=int, default=30, help='Days to analyze (default: 30)')
    parser.add_argument('--min-maintainer-commits', type=int, default=10, help='Minimum commits to be considered maintainer')
    
    args = parser.parse_args()
    
    try:
        owner, name = parse_repo_input(args.repository)
    except ValueError as e:
        print(f"❌ {e}")
        sys.exit(1)
    
    analyzer = GitHubAnalyzer(owner, name, args.days)
    analyzer.min_maintainer_commits = args.min_maintainer_commits
    
    success = analyzer.run_analysis()
    sys.exit(0 if success else 1)

if __name__ == '__main__':
    main()