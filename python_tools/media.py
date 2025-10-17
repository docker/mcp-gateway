#!/usr/bin/env python3
"""
Sonarr & Radarr Auto-Import Tool with Streamlit UI
Interactive frontend for importing TV shows and movies with rating filters.
"""

import streamlit as st
import requests
import time
import os
from typing import List, Dict, Optional
from datetime import datetime
from dotenv import load_dotenv
import json

# Load environment variables from .env file
load_dotenv()

# ==================== BASE API CLASS ====================

class ArrAPI:
    """Base class for Sonarr/Radarr API interactions."""
    
    def __init__(self, url: str, api_key: str, api_type: str):
        """
        Initialize API client.pip
        
        Args:
            url: Base URL of instance
            api_key: API key for authentication
            api_type: 'sonarr' or 'radarr'
        """
        self.url = url.rstrip('/')
        self.api_key = api_key
        self.api_type = api_type
        self.headers = {"X-Api-Key": api_key}
    
    def test_connection(self) -> bool:
        """Test connection to service."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/system/status",
                headers=self.headers,
                timeout=5
            )
            return response.status_code == 200
        except:
            return False
    
    def get_root_folders(self) -> List[Dict]:
        """Get available root folders."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/rootfolder",
                headers=self.headers,
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def get_quality_profiles(self) -> List[Dict]:
        """Get available quality profiles."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/qualityprofile",
                headers=self.headers,
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def _get_or_create_tag(self, tag_label: str) -> int:
        """Get existing tag ID or create new tag."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/tag",
                headers=self.headers,
                timeout=10
            )
            response.raise_for_status()
            tags = response.json()
            
            for tag in tags:
                if tag["label"].lower() == tag_label.lower():
                    return tag["id"]
            
            response = requests.post(
                f"{self.url}/api/v3/tag",
                headers=self.headers,
                json={"label": tag_label},
                timeout=10
            )
            response.raise_for_status()
            return response.json()["id"]
        except:
            return None


# ==================== SONARR API ====================

class SonarrAPI(ArrAPI):
    """Handles Sonarr API interactions."""
    
    def __init__(self, url: str, api_key: str):
        super().__init__(url, api_key, 'sonarr')
    
    def search_series(self, term: str) -> List[Dict]:
        """Search for TV series."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/series/lookup",
                headers=self.headers,
                params={"term": term},
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def get_existing_series(self) -> List[Dict]:
        """Get all series in library."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/series",
                headers=self.headers,
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def add_series(self, series_info: Dict, root_folder: str, quality_profile_id: int,
                   monitor: str, search: bool, tag: str = None) -> Optional[Dict]:
        """Add a series to Sonarr."""
        try:
            tag_id = None
            if tag:
                tag_id = self._get_or_create_tag(tag)
            
            series_data = {
                "title": series_info["title"],
                "qualityProfileId": quality_profile_id,
                "languageProfileId": 1,
                "titleSlug": series_info["titleSlug"],
                "images": series_info.get("images", []),
                "tvdbId": series_info.get("tvdbId", 0),
                "rootFolderPath": root_folder,
                "monitored": True,
                "seasonFolder": True,
                "addOptions": {
                    "monitor": monitor,
                    "searchForMissingEpisodes": search,
                    "searchForCutoffUnmetEpisodes": False
                }
            }
            
            if tag_id:
                series_data["tags"] = [tag_id]
            
            response = requests.post(
                f"{self.url}/api/v3/series",
                headers=self.headers,
                json=series_data,
                timeout=30
            )
            
            if response.status_code == 201:
                return response.json()
            return None
        except:
            return None


# ==================== RADARR API ====================

class RadarrAPI(ArrAPI):
    """Handles Radarr API interactions."""
    
    def __init__(self, url: str, api_key: str):
        super().__init__(url, api_key, 'radarr')
    
    def search_movie(self, term: str) -> List[Dict]:
        """Search for movies."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/movie/lookup",
                headers=self.headers,
                params={"term": term},
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def get_existing_movies(self) -> List[Dict]:
        """Get all movies in library."""
        try:
            response = requests.get(
                f"{self.url}/api/v3/movie",
                headers=self.headers,
                timeout=10
            )
            response.raise_for_status()
            return response.json()
        except:
            return []
    
    def add_movie(self, movie_info: Dict, root_folder: str, quality_profile_id: int,
                  monitor: bool, search: bool, tag: str = None) -> Optional[Dict]:
        """Add a movie to Radarr."""
        try:
            tag_id = None
            if tag:
                tag_id = self._get_or_create_tag(tag)
            
            movie_data = {
                "title": movie_info["title"],
                "qualityProfileId": quality_profile_id,
                "titleSlug": movie_info["titleSlug"],
                "images": movie_info.get("images", []),
                "tmdbId": movie_info.get("tmdbId", 0),
                "year": movie_info.get("year", 0),
                "rootFolderPath": root_folder,
                "monitored": monitor,
                "addOptions": {
                    "searchForMovie": search
                }
            }
            
            if tag_id:
                movie_data["tags"] = [tag_id]
            
            response = requests.post(
                f"{self.url}/api/v3/movie",
                headers=self.headers,
                json=movie_data,
                timeout=30
            )
            
            if response.status_code == 201:
                return response.json()
            return None
        except:
            return None


# ==================== DATA SOURCES ====================

class TraktAPI:
    """Handles Trakt API interactions."""
    
    def __init__(self, client_id: str):
        self.headers = {
            "Content-Type": "application/json",
            "trakt-api-version": "2",
            "trakt-api-key": client_id
        }
    
    def fetch_shows(self, list_type: str, limit: int) -> List[str]:
        """Fetch TV shows from Trakt."""
        try:
            response = requests.get(
                f"https://api.trakt.tv/shows/{list_type}",
                headers=self.headers,
                params={"limit": limit},
                timeout=10
            )
            response.raise_for_status()
            results = response.json()
            
            if list_type == "trending":
                return [item["show"]["title"] for item in results]
            else:
                return [item["title"] for item in results]
        except:
            return []
    
    def fetch_movies(self, list_type: str, limit: int) -> List[str]:
        """Fetch movies from Trakt."""
        try:
            response = requests.get(
                f"https://api.trakt.tv/movies/{list_type}",
                headers=self.headers,
                params={"limit": limit},
                timeout=10
            )
            response.raise_for_status()
            results = response.json()
            
            if list_type == "trending":
                return [item["movie"]["title"] for item in results]
            else:
                return [item["title"] for item in results]
        except:
            return []


# ==================== PREDEFINED LISTS ====================

ROLLING_STONE_TV_TOP_50 = [
    "The Leftovers", "Parks and Recreation", "Breaking Bad",
    "BoJack Horseman", "Fleabag", "The Americans", "Atlanta",
    "Justified", "Rectify", "Better Things", "Better Call Saul",
    "Terriers", "Community", "Twin Peaks", "Game of Thrones",
    "Fargo", "Hannibal", "Halt and Catch Fire", "Bob's Burgers",
    "Review", "Orange Is the New Black", "Crazy Ex-Girlfriend",
    "Brockmire", "Brooklyn Nine-Nine", "Watchmen", "Boardwalk Empire",
    "Veep", "Broad City", "Treme", "Enlightened", "The Good Place",
    "Catastrophe", "Key & Peele", "The Deuce", "The Legend of Korra",
    "You're the Worst", "Master of None", "Pose", "Girls",
    "Steven Universe", "Mr. Robot", "Transparent", "Gravity Falls",
    "Jane the Virgin", "Parenthood", "New Girl", "Men of a Certain Age",
    "Banshee", "Baskets", "The Good Wife"
]

IMDB_TOP_250_MOVIES = [
    "The Shawshank Redemption", "The Godfather", "The Dark Knight",
    "The Godfather Part II", "12 Angry Men", "Schindler's List",
    "The Lord of the Rings: The Return of the King", "Pulp Fiction",
    "The Lord of the Rings: The Fellowship of the Ring", "The Good, the Bad and the Ugly",
    "Forrest Gump", "Fight Club", "Inception", "The Lord of the Rings: The Two Towers",
    "Star Wars: Episode V - The Empire Strikes Back", "The Matrix", "Goodfellas",
    "One Flew Over the Cuckoo's Nest", "Se7en", "Seven Samurai",
    "It's a Wonderful Life", "The Silence of the Lambs", "City of God",
    "Saving Private Ryan", "The Green Mile", "Life Is Beautiful",
    "Interstellar", "Spirited Away", "Parasite", "Léon: The Professional",
    "The Usual Suspects", "Harakiri", "The Lion King", "American History X",
    "Back to the Future", "The Pianist", "Terminator 2: Judgment Day",
    "Modern Times", "Psycho", "Gladiator", "City Lights",
    "The Departed", "Whiplash", "The Prestige", "The Intouchables",
    "Grave of the Fireflies", "Once Upon a Time in the West", "Casablanca",
    "Cinema Paradiso", "Rear Window"
]


# ==================== STREAMLIT UI ====================

def main():
    """Main Streamlit application."""
    
    st.set_page_config(
        page_title="Sonarr & Radarr Auto-Import",
        page_icon="🎬",
        layout="wide"
    )
    
    st.title("🎬 Sonarr & Radarr Auto-Import Tool")
    st.markdown("Import TV shows and movies from multiple sources with rating filters")
    
    # Load environment variables
    sonarr_api_key_env = os.getenv('SONARR_API_KEY')
    radarr_api_key_env = os.getenv('RADARR_API_KEY')
    
    # Show API key status
    if sonarr_api_key_env or radarr_api_key_env:
        st.sidebar.success("🔑 API keys loaded from .env file")
        if sonarr_api_key_env:
            st.sidebar.info(f"📺 Sonarr API key: ***{sonarr_api_key_env[-4:]}")
        if radarr_api_key_env:
            st.sidebar.info(f"🎬 Radarr API key: ***{radarr_api_key_env[-4:]}")
    else:
        st.sidebar.warning("⚠️ No API keys found in .env file")
    
    # Initialize session state
    if 'import_log' not in st.session_state:
        st.session_state.import_log = []
    if 'sonarr_connected' not in st.session_state:
        st.session_state.sonarr_connected = False
    if 'radarr_connected' not in st.session_state:
        st.session_state.radarr_connected = False
    
    # Sidebar - Configuration
    with st.sidebar:
        st.header("⚙️ Configuration")
        
        # Service Selection
        service_type = st.radio(
            "Select Service",
            options=["📺 TV Shows (Sonarr)", "🎬 Movies (Radarr)", "🎭 Both"],
            index=0
        )
        
        use_sonarr = service_type in ["📺 TV Shows (Sonarr)", "🎭 Both"]
        use_radarr = service_type in ["🎬 Movies (Radarr)", "🎭 Both"]
        
        sonarr_api = None
        radarr_api = None
        
        # Sonarr Configuration
        if use_sonarr:
            st.divider()
            st.subheader("📺 Sonarr Settings")
            sonarr_url = st.text_input(
                "Sonarr URL",
                value="http://localhost:8989",
                key="sonarr_url"
            )
            
            # Use API key from .env or allow manual override
            if sonarr_api_key_env:
                use_env_sonarr = st.checkbox("Use API key from .env file", value=True, key="use_env_sonarr")
                if use_env_sonarr:
                    sonarr_api_key = sonarr_api_key_env
                    st.text_input(
                        "Sonarr API Key (from .env)",
                        value=f"***{sonarr_api_key_env[-4:]}",
                        disabled=True,
                        key="sonarr_key_display"
                    )
                else:
                    sonarr_api_key = st.text_input(
                        "Sonarr API Key (manual override)",
                        type="password",
                        key="sonarr_key_manual"
                    )
            else:
                sonarr_api_key = st.text_input(
                    "Sonarr API Key",
                    type="password",
                    key="sonarr_key"
                )
            
            if st.button("🔌 Test Sonarr Connection"):
                if sonarr_api_key:
                    sonarr_api = SonarrAPI(sonarr_url, sonarr_api_key)
                    if sonarr_api.test_connection():
                        st.success("✅ Connected to Sonarr!")
                        st.session_state.sonarr_connected = True
                    else:
                        st.error("❌ Connection failed")
                        st.session_state.sonarr_connected = False
                else:
                    st.warning("Please enter API key")
            
            if st.session_state.sonarr_connected and sonarr_api_key:
                sonarr_api = SonarrAPI(sonarr_url, sonarr_api_key)
                
                root_folders = sonarr_api.get_root_folders()
                if root_folders:
                    sonarr_root = st.selectbox(
                        "Root Folder",
                        options=[f["path"] for f in root_folders],
                        key="sonarr_root"
                    )
                else:
                    sonarr_root = st.text_input("Root Folder", value="/tv", key="sonarr_root_manual")
                
                quality_profiles = sonarr_api.get_quality_profiles()
                if quality_profiles:
                    profile_options = {p["name"]: p["id"] for p in quality_profiles}
                    selected_profile = st.selectbox(
                        "Quality Profile",
                        options=list(profile_options.keys()),
                        key="sonarr_quality"
                    )
                    sonarr_quality_id = profile_options[selected_profile]
                else:
                    sonarr_quality_id = 1
                
                sonarr_monitor = st.selectbox(
                    "Monitor",
                    options=["all", "future", "missing", "existing", "firstSeason", "latestSeason", "none"],
                    index=1,
                    key="sonarr_monitor"
                )
                
                sonarr_search = st.checkbox("Search immediately", value=False, key="sonarr_search")
                sonarr_tag = st.text_input("Tag", value="auto-imported", key="sonarr_tag")
        
        # Radarr Configuration
        if use_radarr:
            st.divider()
            st.subheader("🎬 Radarr Settings")
            radarr_url = st.text_input(
                "Radarr URL",
                value="http://localhost:7878",
                key="radarr_url"
            )
            
            # Use API key from .env or allow manual override
            if radarr_api_key_env:
                use_env_radarr = st.checkbox("Use API key from .env file", value=True, key="use_env_radarr")
                if use_env_radarr:
                    radarr_api_key = radarr_api_key_env
                    st.text_input(
                        "Radarr API Key (from .env)",
                        value=f"***{radarr_api_key_env[-4:]}",
                        disabled=True,
                        key="radarr_key_display"
                    )
                else:
                    radarr_api_key = st.text_input(
                        "Radarr API Key (manual override)",
                        type="password",
                        key="radarr_key_manual"
                    )
            else:
                radarr_api_key = st.text_input(
                    "Radarr API Key",
                    type="password",
                    key="radarr_key"
                )
            
            if st.button("🔌 Test Radarr Connection"):
                if radarr_api_key:
                    radarr_api = RadarrAPI(radarr_url, radarr_api_key)
                    if radarr_api.test_connection():
                        st.success("✅ Connected to Radarr!")
                        st.session_state.radarr_connected = True
                    else:
                        st.error("❌ Connection failed")
                        st.session_state.radarr_connected = False
                else:
                    st.warning("Please enter API key")
            
            if st.session_state.radarr_connected and radarr_api_key:
                radarr_api = RadarrAPI(radarr_url, radarr_api_key)
                
                root_folders = radarr_api.get_root_folders()
                if root_folders:
                    radarr_root = st.selectbox(
                        "Root Folder",
                        options=[f["path"] for f in root_folders],
                        key="radarr_root"
                    )
                else:
                    radarr_root = st.text_input("Root Folder", value="/movies", key="radarr_root_manual")
                
                quality_profiles = radarr_api.get_quality_profiles()
                if quality_profiles:
                    profile_options = {p["name"]: p["id"] for p in quality_profiles}
                    selected_profile = st.selectbox(
                        "Quality Profile",
                        options=list(profile_options.keys()),
                        key="radarr_quality"
                    )
                    radarr_quality_id = profile_options[selected_profile]
                else:
                    radarr_quality_id = 1
                
                radarr_monitor = st.checkbox("Monitor movies", value=True, key="radarr_monitor")
                radarr_search = st.checkbox("Search immediately", value=False, key="radarr_search")
                radarr_tag = st.text_input("Tag", value="auto-imported", key="radarr_tag")
        
        # Rating Filters
        st.divider()
        st.subheader("⭐ Rating Filters")
        min_rating = st.slider(
            "Minimum IMDb Rating",
            min_value=0.0,
            max_value=10.0,
            value=7.5,
            step=0.1
        )
        
        min_votes = st.number_input(
            "Minimum Votes",
            min_value=0,
            max_value=1000000,
            value=10000,
            step=1000
        )
        
        # Year Filters
        st.divider()
        st.subheader("📅 Year Filters")
        
        year_filter_type = st.radio(
            "Year Filter Type",
            options=["Range", "Specific Years", "Decades", "None"],
            index=0,
            help="Choose how to filter by release year"
        )
        
        year_filter = {}
        
        if year_filter_type == "Range":
            col_year1, col_year2 = st.columns(2)
            with col_year1:
                min_year = st.number_input(
                    "From Year",
                    min_value=1900,
                    max_value=datetime.now().year,
                    value=2000
                )
            with col_year2:
                max_year = st.number_input(
                    "To Year",
                    min_value=1900,
                    max_value=datetime.now().year + 5,
                    value=datetime.now().year
                )
            year_filter = {'type': 'range', 'min': min_year, 'max': max_year}
        
        elif year_filter_type == "Specific Years":
            specific_years = st.text_input(
                "Enter years (comma-separated)",
                value="2020, 2021, 2022, 2023",
                help="e.g., 2020, 2021, 2022"
            )
            year_list = [int(y.strip()) for y in specific_years.split(',') if y.strip().isdigit()]
            year_filter = {'type': 'specific', 'years': year_list}
        
        elif year_filter_type == "Decades":
            decades = st.multiselect(
                "Select Decades",
                options=['1950s', '1960s', '1970s', '1980s', '1990s', '2000s', '2010s', '2020s'],
                default=['2010s', '2020s']
            )
            year_filter = {'type': 'decades', 'decades': decades}
        
        else:
            year_filter = {'type': 'none'}
        
        # Genre Filters
        st.divider()
        st.subheader("🎭 Genre Filters")
        
        enable_genre_filter = st.checkbox("Enable Genre Filtering", value=False)
        
        genre_filter = {}
        
        if enable_genre_filter:
            genre_mode = st.radio(
                "Genre Filter Mode",
                options=["Include (must have at least one)", "Exclude (must not have any)"],
                index=0
            )
            
            # Common genres for both TV and movies
            all_genres = [
                'Action', 'Adventure', 'Animation', 'Biography', 'Comedy',
                'Crime', 'Documentary', 'Drama', 'Family', 'Fantasy',
                'Film-Noir', 'History', 'Horror', 'Music', 'Musical',
                'Mystery', 'Romance', 'Sci-Fi', 'Sport', 'Thriller',
                'War', 'Western'
            ]
            
            selected_genres = st.multiselect(
                "Select Genres",
                options=all_genres,
                default=[]
            )
            
            genre_filter = {
                'enabled': True,
                'mode': 'include' if genre_mode.startswith('Include') else 'exclude',
                'genres': [g.lower() for g in selected_genres]
            }
        else:
            genre_filter = {'enabled': False}
    
    # Main content area
    if use_sonarr and not st.session_state.sonarr_connected:
        st.info("👈 Please configure and test Sonarr connection in the sidebar")
        return
    
    if use_radarr and not st.session_state.radarr_connected:
        st.info("👈 Please configure and test Radarr connection in the sidebar")
        return
    
    # Data Source Selection
    st.header("📊 Select Data Sources")
    
    # TV Shows Section
    if use_sonarr:
        st.subheader("📺 TV Show Sources")
        
        col1, col2 = st.columns(2)
        
        with col1:
            st.markdown("**Curated Lists**")
            
            use_rolling_stone = st.checkbox(
                "Rolling Stone Top 50 (2010s)",
                value=True,
                key="use_rs"
            )
            if use_rolling_stone:
                rs_limit = st.slider(
                    "Number of shows",
                    min_value=1,
                    max_value=50,
                    value=50,
                    key="rs_limit"
                )
            
            use_custom_tv = st.checkbox("Custom TV Show List", key="use_custom_tv")
            if use_custom_tv:
                custom_tv_shows = st.text_area(
                    "Enter show titles (one per line)",
                    height=150,
                    key="custom_tv",
                    placeholder="Breaking Bad\nThe Wire\nGame of Thrones"
                )
        
        with col2:
            st.markdown("**Trakt.tv TV Shows**")
            
            use_trakt_tv = st.checkbox("Enable Trakt.tv for TV", key="use_trakt_tv")
            
            if use_trakt_tv:
                trakt_client_id = st.text_input(
                    "Trakt Client ID",
                    type="password",
                    key="trakt_id"
                )
                
                if trakt_client_id:
                    trakt_tv_trending = st.checkbox("Trending TV Shows", key="trakt_tv_trend")
                    if trakt_tv_trending:
                        tv_trending_limit = st.slider(
                            "Number of trending shows",
                            1, 100, 25,
                            key="tv_trend_limit"
                        )
                    
                    trakt_tv_popular = st.checkbox("Popular TV Shows", key="trakt_tv_pop")
                    if trakt_tv_popular:
                        tv_popular_limit = st.slider(
                            "Number of popular shows",
                            1, 100, 25,
                            key="tv_pop_limit"
                        )
                    
                    trakt_tv_anticipated = st.checkbox("Anticipated TV Shows", key="trakt_tv_ant")
                    if trakt_tv_anticipated:
                        tv_anticipated_limit = st.slider(
                            "Number of anticipated shows",
                            1, 100, 25,
                            key="tv_ant_limit"
                        )
    
    # Movies Section
    if use_radarr:
        st.divider()
        st.subheader("🎬 Movie Sources")
        
        col1, col2 = st.columns(2)
        
        with col1:
            st.markdown("**Curated Lists**")
            
            use_imdb_top = st.checkbox(
                "IMDb Top 250 Movies",
                value=True,
                key="use_imdb"
            )
            if use_imdb_top:
                imdb_limit = st.slider(
                    "Number of movies",
                    min_value=1,
                    max_value=50,
                    value=50,
                    key="imdb_limit"
                )
            
            use_custom_movies = st.checkbox("Custom Movie List", key="use_custom_movies")
            if use_custom_movies:
                custom_movies = st.text_area(
                    "Enter movie titles (one per line)",
                    height=150,
                    key="custom_movies",
                    placeholder="The Shawshank Redemption\nThe Godfather\nThe Dark Knight"
                )
        
        with col2:
            st.markdown("**Trakt.tv Movies**")
            
            use_trakt_movies = st.checkbox("Enable Trakt.tv for Movies", key="use_trakt_movies")
            
            if use_trakt_movies:
                trakt_movie_client_id = st.text_input(
                    "Trakt Client ID",
                    type="password",
                    key="trakt_movie_id",
                    value=trakt_client_id if use_trakt_tv and trakt_client_id else ""
                )
                
                if trakt_movie_client_id:
                    trakt_movie_trending = st.checkbox("Trending Movies", key="trakt_movie_trend")
                    if trakt_movie_trending:
                        movie_trending_limit = st.slider(
                            "Number of trending movies",
                            1, 100, 25,
                            key="movie_trend_limit"
                        )
                    
                    trakt_movie_popular = st.checkbox("Popular Movies", key="trakt_movie_pop")
                    if trakt_movie_popular:
                        movie_popular_limit = st.slider(
                            "Number of popular movies",
                            1, 100, 25,
                            key="movie_pop_limit"
                        )
                    
                    trakt_movie_anticipated = st.checkbox("Anticipated Movies", key="trakt_movie_ant")
                    if trakt_movie_anticipated:
                        movie_anticipated_limit = st.slider(
                            "Number of anticipated movies",
                            1, 100, 25,
                            key="movie_ant_limit"
                        )
    
    st.divider()
    
    # Import Button
    col1, col2, col3 = st.columns([1, 2, 1])
    with col2:
        if st.button("🚀 Start Import", type="primary", use_container_width=True):
            # Collect configuration
            config = {
                'use_sonarr': use_sonarr,
                'use_radarr': use_radarr,
                'min_rating': min_rating,
                'min_votes': min_votes,
                'year_filter': year_filter,
                'genre_filter': genre_filter
            }
            
            if use_sonarr:
                config['sonarr'] = {
                    'api': sonarr_api,
                    'root': sonarr_root,
                    'quality_id': sonarr_quality_id,
                    'monitor': sonarr_monitor,
                    'search': sonarr_search,
                    'tag': sonarr_tag,
                    'sources': {}
                }
                
                if use_rolling_stone:
                    config['sonarr']['sources']['rolling_stone'] = rs_limit
                if use_custom_tv:
                    config['sonarr']['sources']['custom'] = custom_tv_shows
                if use_trakt_tv and trakt_client_id:
                    config['sonarr']['sources']['trakt'] = {
                        'client_id': trakt_client_id,
                        'trending': tv_trending_limit if trakt_tv_trending else 0,
                        'popular': tv_popular_limit if trakt_tv_popular else 0,
                        'anticipated': tv_anticipated_limit if trakt_tv_anticipated else 0
                    }
            
            if use_radarr:
                config['radarr'] = {
                    'api': radarr_api,
                    'root': radarr_root,
                    'quality_id': radarr_quality_id,
                    'monitor': radarr_monitor,
                    'search': radarr_search,
                    'tag': radarr_tag,
                    'sources': {}
                }
                
                if use_imdb_top:
                    config['radarr']['sources']['imdb_top'] = imdb_limit
                if use_custom_movies:
                    config['radarr']['sources']['custom'] = custom_movies
                if use_trakt_movies and trakt_movie_client_id:
                    config['radarr']['sources']['trakt'] = {
                        'client_id': trakt_movie_client_id,
                        'trending': movie_trending_limit if trakt_movie_trending else 0,
                        'popular': movie_popular_limit if trakt_movie_popular else 0,
                        'anticipated': movie_anticipated_limit if trakt_movie_anticipated else 0
                    }
            
            run_import(config)
    
    # Display import log
    if st.session_state.import_log:
        st.divider()
        st.header("📋 Import Log")
        
        for log_entry in st.session_state.import_log:
            if log_entry['type'] == 'success':
                st.success(log_entry['message'])
            elif log_entry['type'] == 'warning':
                st.warning(log_entry['message'])
            elif log_entry['type'] == 'error':
                st.error(log_entry['message'])
            else:
                st.info(log_entry['message'])


def run_import(config):
    """Execute the import process."""
    
    st.session_state.import_log = []
    
    # Import TV Shows
    if config['use_sonarr']:
        import_tv_shows(config)
    
    # Import Movies
    if config['use_radarr']:
        import_movies(config)


def apply_filters(item_info: Dict, config: Dict) -> tuple[bool, str]:
    """
    Apply rating, year, and genre filters to a show/movie.
    
    Args:
        item_info: Series or movie information from Sonarr/Radarr
        config: Configuration with filter settings
        
    Returns:
        Tuple of (passes_filter, reason_if_failed)
    """
    # Rating filter
    if item_info.get("ratings"):
        rating_value = item_info["ratings"].get("value", 0)
        rating_votes = item_info["ratings"].get("votes", 0)
        
        if rating_value < config['min_rating']:
            return False, f"Rating too low: {rating_value:.1f} < {config['min_rating']}"
        
        if rating_votes < config['min_votes']:
            return False, f"Not enough votes: {rating_votes:,} < {config['min_votes']:,}"
    
    # Year filter
    year = item_info.get("year", 0)
    if year and config['year_filter']['type'] != 'none':
        year_filter = config['year_filter']
        
        if year_filter['type'] == 'range':
            if year < year_filter['min'] or year > year_filter['max']:
                return False, f"Year {year} outside range {year_filter['min']}-{year_filter['max']}"
        
        elif year_filter['type'] == 'specific':
            if year not in year_filter['years']:
                return False, f"Year {year} not in specified years"
        
        elif year_filter['type'] == 'decades':
            decade_start = (year // 10) * 10
            decade_str = f"{decade_start}s"
            if decade_str not in year_filter['decades']:
                return False, f"Year {year} ({decade_str}) not in selected decades"
    
    # Genre filter
    if config['genre_filter']['enabled']:
        genres = [g.lower() for g in item_info.get("genres", [])]
        filter_genres = config['genre_filter']['genres']
        
        if not filter_genres:  # No genres selected, skip filter
            pass
        elif config['genre_filter']['mode'] == 'include':
            # Must have at least one of the selected genres
            if not any(g in genres for g in filter_genres):
                return False, f"No matching genres (needs one of: {', '.join(filter_genres)})"
        else:  # exclude mode
            # Must not have any of the selected genres
            if any(g in genres for g in filter_genres):
                matching = [g for g in filter_genres if g in genres]
                return False, f"Excluded genre found: {', '.join(matching)}"
    
    return True, ""


def import_tv_shows(config):
    """Import TV shows to Sonarr."""
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': "📺 Starting TV Show Import..."
    })
    
    sonarr = config['sonarr']['api']
    sources = config['sonarr']['sources']
    
    # Load existing
    existing = sonarr.get_existing_series()
    existing_titles = {show["title"].lower() for show in existing}
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': f"Found {len(existing_titles)} existing TV shows"
    })
    
    # Collect shows
    all_shows = []
    
    if 'rolling_stone' in sources:
        all_shows.extend(ROLLING_STONE_TV_TOP_50[:sources['rolling_stone']])
        st.session_state.import_log.append({
            'type': 'info',
            'message': f"Added {sources['rolling_stone']} shows from Rolling Stone"
        })
    
    if 'custom' in sources:
        custom = [s.strip() for s in sources['custom'].split('\n') if s.strip()]
        all_shows.extend(custom)
        st.session_state.import_log.append({
            'type': 'info',
            'message': f"Added {len(custom)} custom TV shows"
        })
    
    if 'trakt' in sources:
        trakt = TraktAPI(sources['trakt']['client_id'])
        if sources['trakt']['trending'] > 0:
            trending = trakt.fetch_shows('trending', sources['trakt']['trending'])
            all_shows.extend(trending)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(trending)} trending TV shows from Trakt"
            })
        if sources['trakt']['popular'] > 0:
            popular = trakt.fetch_shows('popular', sources['trakt']['popular'])
            all_shows.extend(popular)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(popular)} popular TV shows from Trakt"
            })
        if sources['trakt']['anticipated'] > 0:
            anticipated = trakt.fetch_shows('anticipated', sources['trakt']['anticipated'])
            all_shows.extend(anticipated)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(anticipated)} anticipated TV shows from Trakt"
            })
    
    # Remove duplicates
    all_shows = list(dict.fromkeys(all_shows))
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': f"Processing {len(all_shows)} unique TV shows..."
    })
    
    # Process shows
    added = 0
    skipped = 0
    failed = 0
    filtered_out = 0
    
    progress_bar = st.progress(0)
    status_text = st.empty()
    
    for i, title in enumerate(all_shows):
        progress_bar.progress((i + 1) / len(all_shows))
        status_text.text(f"📺 Processing {i+1}/{len(all_shows)}: {title}")
        
        if title.lower() in existing_titles:
            skipped += 1
            continue
        
        results = sonarr.search_series(title)
        if not results:
            failed += 1
            continue
        
        series_info = results[0]
        
        # Apply filters
        passes, reason = apply_filters(series_info, config)
        if not passes:
            filtered_out += 1
            continue
        
        result = sonarr.add_series(
            series_info,
            config['sonarr']['root'],
            config['sonarr']['quality_id'],
            config['sonarr']['monitor'],
            config['sonarr']['search'],
            config['sonarr']['tag']
        )
        
        if result:
            added += 1
            existing_titles.add(title.lower())
        else:
            skipped += 1
        
        time.sleep(0.5)
    
    progress_bar.empty()
    status_text.empty()
    
    st.session_state.import_log.append({
        'type': 'success',
        'message': f"📺 TV Shows - Added: {added} | Skipped: {skipped} | Filtered: {filtered_out} | Failed: {failed}"
    })


def import_movies(config):
    """Import movies to Radarr."""
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': "🎬 Starting Movie Import..."
    })
    
    radarr = config['radarr']['api']
    sources = config['radarr']['sources']
    
    # Load existing
    existing = radarr.get_existing_movies()
    existing_titles = {movie["title"].lower() for movie in existing}
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': f"Found {len(existing_titles)} existing movies"
    })
    
    # Collect movies
    all_movies = []
    
    if 'imdb_top' in sources:
        all_movies.extend(IMDB_TOP_250_MOVIES[:sources['imdb_top']])
        st.session_state.import_log.append({
            'type': 'info',
            'message': f"Added {sources['imdb_top']} movies from IMDb Top 250"
        })
    
    if 'custom' in sources:
        custom = [s.strip() for s in sources['custom'].split('\n') if s.strip()]
        all_movies.extend(custom)
        st.session_state.import_log.append({
            'type': 'info',
            'message': f"Added {len(custom)} custom movies"
        })
    
    if 'trakt' in sources:
        trakt = TraktAPI(sources['trakt']['client_id'])
        if sources['trakt']['trending'] > 0:
            trending = trakt.fetch_movies('trending', sources['trakt']['trending'])
            all_movies.extend(trending)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(trending)} trending movies from Trakt"
            })
        if sources['trakt']['popular'] > 0:
            popular = trakt.fetch_movies('popular', sources['trakt']['popular'])
            all_movies.extend(popular)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(popular)} popular movies from Trakt"
            })
        if sources['trakt']['anticipated'] > 0:
            anticipated = trakt.fetch_movies('anticipated', sources['trakt']['anticipated'])
            all_movies.extend(anticipated)
            st.session_state.import_log.append({
                'type': 'info',
                'message': f"Added {len(anticipated)} anticipated movies from Trakt"
            })
    
    # Remove duplicates
    all_movies = list(dict.fromkeys(all_movies))
    
    st.session_state.import_log.append({
        'type': 'info',
        'message': f"Processing {len(all_movies)} unique movies..."
    })
    
    # Process movies
    added = 0
    skipped = 0
    failed = 0
    filtered_out = 0
    
    progress_bar = st.progress(0)
    status_text = st.empty()
    
    for i, title in enumerate(all_movies):
        progress_bar.progress((i + 1) / len(all_movies))
        status_text.text(f"🎬 Processing {i+1}/{len(all_movies)}: {title}")
        
        if title.lower() in existing_titles:
            skipped += 1
            continue
        
        results = radarr.search_movie(title)
        if not results:
            failed += 1
            continue
        
        movie_info = results[0]
        
        # Apply filters
        passes, reason = apply_filters(movie_info, config)
        if not passes:
            filtered_out += 1
            continue
        
        result = radarr.add_movie(
            movie_info,
            config['radarr']['root'],
            config['radarr']['quality_id'],
            config['radarr']['monitor'],
            config['radarr']['search'],
            config['radarr']['tag']
        )
        
        if result:
            added += 1
            existing_titles.add(title.lower())
        else:
            skipped += 1
        
        time.sleep(0.5)
    
    progress_bar.empty()
    status_text.empty()
    
    st.session_state.import_log.append({
        'type': 'success',
        'message': f"🎬 Movies - Added: {added} | Skipped: {skipped} | Filtered: {filtered_out} | Failed: {failed}"
    })


if __name__ == "__main__":
    main()