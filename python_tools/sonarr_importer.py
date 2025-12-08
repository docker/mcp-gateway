#!/usr/bin/env python3
"""
Sonarr Bulk TV Show Importer
Adds a list of TV shows to Sonarr for monitoring and searching.
"""

import requests
import time
from typing import List, Dict, Optional

# Configuration - UPDATE THESE VALUES
SONARR_URL = "http://localhost:8989"  # Your Sonarr URL
API_KEY = "YOUR_API_KEY_HERE"  # Your Sonarr API key
ROOT_FOLDER = "/tv"  # Your root folder path in Sonarr
QUALITY_PROFILE_ID = 1  # Your quality profile ID (1 is usually default)
LANGUAGE_PROFILE_ID = 1  # Your language profile ID

# Top 50 TV shows from Rolling Stone's 2010s list
TV_SHOWS = [
    "The Leftovers",
    "Parks and Recreation",
    "Breaking Bad",
    "BoJack Horseman",
    "Fleabag",
    "The Americans",
    "Atlanta",
    "Justified",
    "Rectify",
    "Better Things",
    "Better Call Saul",
    "Terriers",
    "Community",
    "Twin Peaks: The Return",
    "Game of Thrones",
    "Fargo",
    "Hannibal",
    "Halt and Catch Fire",
    "Bob's Burgers",
    "Review",
    "Orange Is the New Black",
    "Crazy Ex-Girlfriend",
    "Brockmire",
    "Brooklyn Nine-Nine",
    "Watchmen",
    "Boardwalk Empire",
    "Veep",
    "Broad City",
    "Treme",
    "Enlightened",
    "The Good Place",
    "Catastrophe",
    "Key & Peele",
    "The Deuce",
    "The Legend of Korra",
    "You're the Worst",
    "Master of None",
    "Pose",
    "Girls",
    "Steven Universe",
    "Mr. Robot",
    "Transparent",
    "Gravity Falls",
    "Jane the Virgin",
    "Parenthood",
    "New Girl",
    "Men of a Certain Age",
    "Banshee",
    "Baskets",
    "The Good Wife"
]


class SonarrImporter:
    """Handles bulk importing TV shows into Sonarr."""
    
    def __init__(self, url: str, api_key: str, root_folder: str, 
                 quality_profile_id: int, language_profile_id: int):
        """
        Initialize the Sonarr importer.
        
        Args:
            url: Base URL of your Sonarr instance
            api_key: API key for authentication
            root_folder: Root folder path for TV shows
            quality_profile_id: ID of the quality profile to use
            language_profile_id: ID of the language profile to use
        """
        self.url = url.rstrip('/')
        self.api_key = api_key
        self.root_folder = root_folder
        self.quality_profile_id = quality_profile_id
        self.language_profile_id = language_profile_id
        self.headers = {"X-Api-Key": api_key}
    
    def search_series(self, title: str) -> Optional[Dict]:
        """
        Search for a TV series in Sonarr's lookup.
        
        Args:
            title: Title of the TV show to search for
            
        Returns:
            Dictionary with series info if found, None otherwise
        """
        try:
            response = requests.get(
                f"{self.url}/api/v3/series/lookup",
                headers=self.headers,
                params={"term": title},
                timeout=10
            )
            response.raise_for_status()
            results = response.json()
            
            if results:
                # Return the first (best) match
                return results[0]
            return None
            
        except Exception as e:
            print(f"Error searching for '{title}': {e}")
            return None
    
    def add_series(self, series_info: Dict, monitor: bool = True, 
                   search: bool = True) -> bool:
        """
        Add a series to Sonarr.
        
        Args:
            series_info: Series information from lookup
            monitor: Whether to monitor the series
            search: Whether to search for episodes immediately
            
        Returns:
            True if successful, False otherwise
        """
        try:
            # Prepare the series data for adding
            series_data = {
                "title": series_info["title"],
                "qualityProfileId": self.quality_profile_id,
                "languageProfileId": self.language_profile_id,
                "titleSlug": series_info["titleSlug"],
                "images": series_info.get("images", []),
                "tvdbId": series_info.get("tvdbId", 0),
                "rootFolderPath": self.root_folder,
                "monitored": monitor,
                "addOptions": {
                    "searchForMissingEpisodes": search
                }
            }
            
            response = requests.post(
                f"{self.url}/api/v3/series",
                headers=self.headers,
                json=series_data,
                timeout=30
            )
            
            if response.status_code == 201:
                return True
            elif response.status_code == 400:
                error = response.json()
                if "SeriesExistsValidator" in str(error):
                    print(f"  Already exists in Sonarr")
                    return False
                else:
                    print(f"  Error: {error}")
                    return False
            else:
                print(f"  HTTP {response.status_code}: {response.text}")
                return False
                
        except Exception as e:
            print(f"  Error adding series: {e}")
            return False
    
    def import_shows(self, show_list: List[str], delay: float = 1.0):
        """
        Import a list of TV shows into Sonarr.
        
        Args:
            show_list: List of TV show titles to import
            delay: Delay between requests in seconds (to avoid rate limiting)
        """
        added = 0
        skipped = 0
        failed = 0
        
        print(f"Starting import of {len(show_list)} shows...\n")
        
        for i, show_title in enumerate(show_list, 1):
            print(f"[{i}/{len(show_list)}] Processing: {show_title}")
            
            # Search for the series
            series_info = self.search_series(show_title)
            
            if not series_info:
                print(f"  ❌ Not found in Sonarr database")
                failed += 1
                continue
            
            # Add the series
            if self.add_series(series_info):
                print(f"  ✅ Added successfully")
                added += 1
            else:
                skipped += 1
            
            # Rate limiting delay
            if i < len(show_list):
                time.sleep(delay)
        
        # Print summary
        print(f"\n{'='*50}")
        print(f"Import Summary:")
        print(f"  ✅ Added: {added}")
        print(f"  ⏭️  Skipped (already exists): {skipped}")
        print(f"  ❌ Failed: {failed}")
        print(f"{'='*50}")


def main():
    """Main function to run the importer."""
    
    # Validate configuration
    if API_KEY == "YOUR_API_KEY_HERE":
        print("❌ Error: Please update the API_KEY in the script!")
        print("\nTo find your API key:")
        print("1. Open Sonarr web interface")
        print("2. Go to Settings → General")
        print("3. Click 'Show Advanced'")
        print("4. Copy the API Key")
        return
    
    # Initialize importer
    importer = SonarrImporter(
        url=SONARR_URL,
        api_key=API_KEY,
        root_folder=ROOT_FOLDER,
        quality_profile_id=QUALITY_PROFILE_ID,
        language_profile_id=LANGUAGE_PROFILE_ID
    )
    
    # Import shows
    importer.import_shows(TV_SHOWS)


if __name__ == "__main__":
    main()
