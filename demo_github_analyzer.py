#!/usr/bin/env python3
"""
GitHub Repository Contribution Analysis - Demo Version
Shows how the analysis would work with sample data
"""

import json
from datetime import datetime
from collections import defaultdict, Counter

def demo_analysis():
    """Demonstrate the analysis with sample data"""
    
    # Sample data representing a typical repository
    sample_commits = [
        {"author": {"login": "maintainer1"}, "commit": {"message": "Add new feature"}},
        {"author": {"login": "maintainer1"}, "commit": {"message": "Fix bug in parser"}},
        {"author": {"login": "maintainer2"}, "commit": {"message": "Update documentation"}},
        {"author": {"login": "contributor1"}, "commit": {"message": "Merge pull request #45"}},
        {"author": {"login": "contributor2"}, "commit": {"message": "Add tests"}},
        {"author": {"login": "maintainer1"}, "commit": {"message": "Merge pull request #46"}},
        {"author": {"login": "contributor3"}, "commit": {"message": "Fix typo"}},
        {"author": {"login": "maintainer2"}, "commit": {"message": "Release v1.2.0"}},
        {"author": {"login": "contributor1"}, "commit": {"message": "Update README"}},
        {"author": {"login": "maintainer1"}, "commit": {"message": "Merge pull request #47"}},
    ]
    
    sample_prs = [
        {"author": {"login": "contributor1"}, "number": 45, "title": "Add JSON support", "state": "merged"},
        {"author": {"login": "contributor2"}, "number": 46, "title": "Improve test coverage", "state": "merged"},
        {"author": {"login": "contributor3"}, "number": 47, "title": "Fix documentation typo", "state": "merged"},
        {"author": {"login": "contributor1"}, "number": 48, "title": "Add CLI interface", "state": "merged"},
        {"author": {"login": "contributor4"}, "number": 49, "title": "Performance optimization", "state": "open"},
        {"author": {"login": "contributor5"}, "number": 50, "title": "Add Docker support", "state": "merged"},
        {"author": {"login": "contributor1"}, "number": 51, "title": "Bug fixes", "state": "closed"},
        {"author": {"login": "contributor6"}, "number": 52, "title": "Update dependencies", "state": "merged"},
    ]
    
    # Simulate the analysis
    repo_full = "example/awesome-project"
    days = 30
    min_maintainer_commits = 3
    
    # Count commits by author
    commit_authors = defaultdict(int)
    for commit in sample_commits:
        author = commit.get('author', {}).get('login', 'Unknown')
        commit_authors[author] += 1
    
    # Count PRs by author
    pr_authors = defaultdict(int)
    for pr in sample_prs:
        author = pr.get('author', {}).get('login', 'Unknown')
        pr_authors[author] += 1
    
    # Identify maintainers (high commit count)
    maintainers = set()
    for author, count in commit_authors.items():
        if count >= min_maintainer_commits:
            maintainers.add(author)
    
    # Calculate non-maintainer statistics
    non_maintainer_prs = [pr for pr in sample_prs if pr.get('author', {}).get('login') not in maintainers]
    non_maintainer_counts = Counter(pr.get('author', {}).get('login') for pr in non_maintainer_prs)
    
    # Generate report
    total_prs = len(sample_prs)
    
    print("="*80)
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
    
    print(f"\n🏆 TOP NON-MAINTAINERS BY PR COUNT:")
    print("-" * 70)
    print(f"{'Rank':<6} {'Username':<25} {'PR Count':<10} {'Contribution %':<15}")
    print("-" * 70)
    
    sorted_contributors = sorted(non_maintainer_counts.items(), key=lambda x: x[1], reverse=True)
    
    for rank, (username, count) in enumerate(sorted_contributors, 1):
        percentage = (count / total_prs * 100) if total_prs > 0 else 0
        print(f"{rank:<6} {username:<25} {count:<10} {percentage:<15.1f}%")
    
    print(f"\n👥 IDENTIFIED MAINTAINERS:")
    print("-" * 40)
    maintainer_list = sorted(list(maintainers))
    for i, maintainer in enumerate(maintainer_list, 1):
        commits = commit_authors.get(maintainer, 0)
        prs = pr_authors.get(maintainer, 0)
        print(f"{i:>3}. {maintainer} ({commits} commits, {prs} PRs)")
    
    print(f"\n💡 ANALYSIS INSIGHTS:")
    print("-" * 25)
    print(f"• Average PRs per non-maintainer: {len(non_maintainer_prs) / len(non_maintainer_counts):.1f}")
    print(f"• Most active contributor: {sorted_contributors[0][0]} with {sorted_contributors[0][1]} PRs")
    print(f"• Community engagement: Strong ({len(non_maintainer_counts)} unique contributors)")
    
    print(f"\n🔍 MERGE COMMIT ANALYSIS:")
    print("-" * 30)
    merge_commits = [c for c in sample_commits if "Merge pull request" in c['commit']['message']]
    print(f"• Total merge commits: {len(merge_commits)}")
    print(f"• Maintainers handling merges: {len(set(c['author']['login'] for c in merge_commits))}")
    
    print("\n" + "="*80)
    print("✅ Analysis complete! This demonstrates how the tool analyzes:")
    print("   • Commit patterns and authorship")
    print("   • Pull request contributions")
    print("   • Maintainer vs contributor identification")
    print("   • Community engagement metrics")
    
    # Sample JSON output
    analysis_data = {
        'repository': repo_full,
        'analysis_date': datetime.now().isoformat(),
        'period_days': days,
        'maintainers': list(maintainers),
        'non_maintainer_counts': dict(non_maintainer_counts),
        'total_prs': total_prs,
        'non_maintainer_prs_count': len(non_maintainer_prs),
        'community_metrics': {
            'unique_contributors': len(non_maintainer_counts),
            'average_prs_per_contributor': len(non_maintainer_prs) / len(non_maintainer_counts),
            'contribution_percentage': (len(non_maintainer_prs) / total_prs) * 100
        }
    }
    
    filename = f"demo_analysis_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(filename, 'w') as f:
        json.dump(analysis_data, f, indent=2)
    
    print(f"\n💾 Sample data saved to: {filename}")
    
    return analysis_data

if __name__ == '__main__':
    demo_analysis()