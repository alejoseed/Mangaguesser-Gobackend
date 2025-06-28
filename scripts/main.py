import csv
import requests
from lxml import html
import re
import os
from playwright.sync_api import sync_playwright, Playwright
import polars as pl
from fuzzywuzzy import fuzz
import random
import time
import urllib3

import certifi
import ssl

# const pathToExtension = require('path').join(__dirname, 'my-extension');

ublock = "/Users/alejoseed/Projects/Mangaguesser-Gobackend/scripts/uBlock0.chromium"
# ublock = "/Users/alejoseed/Projects/Mangaguesser-Gobackend/scripts/uBlock0_1.64.1b8.firefox.signed.xpi"

user_data_dir = "/tmp/test-user-data-dir"

http = urllib3.PoolManager(
    cert_reqs='CERT_REQUIRED',
    ca_certs=certifi.where()
)

from pathlib import Path
from requests.exceptions import RequestException

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def create_initial_csv():
    base_url = "https://myanimelist.net/topmanga.php"
    headers = {"User-Agent": "Mozilla/5.0"}

    with open("top_manga.csv", "w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(["Rank", "English Title", "Japanese Title", "Link"])

        offset = 0
        max_offset = 1000
        step = 50
        rank = 1

        while offset <= max_offset:
            params = {"type": "manga", "limit": offset}
            response = requests.get(base_url, headers=headers, params=params)
            response.raise_for_status()

            tree = html.fromstring(response.content)
            elements = tree.xpath('//h3[@class="manga_h3"]/a')

            if not elements:
                break

            for el in elements:
                english_title = el.text_content().strip()
                link = el.get("href")
                japanese_title = ""

                try:
                    manga_page = requests.get(link, headers=headers)
                    manga_page.encoding = 'utf-8'
                    manga_tree = html.fromstring(manga_page.text)
                    jp_node = manga_tree.xpath(
                        '//div[@class="spaceit_pad"]/span[text()="Japanese:"]/following-sibling::text()'
                    )
                    if jp_node:
                        japanese_title = jp_node[0].strip()
                except Exception as e:
                    print(f"⚠️ Failed to fetch Japanese title for {english_title}: {e}")

                writer.writerow([rank, english_title, japanese_title, link])
                print(f"✅ {rank}. {english_title} ({japanese_title})")
                rank += 1

                time.sleep(1.5)

            offset += step
            time.sleep(2)

def create_folders_from_csv():
    with open("top_manga.csv", "r", newline="", encoding="utf-8") as f:
        file_content = csv.reader(f)
        for line in file_content:

            title = line[1]
            folder_name = sanitize_folder_name(title)

            try:
                os.makedirs(f"mangas/{folder_name}", exist_ok=True)
                print(f"Created folder {folder_name}")
            except Exception as e:
                print(e)

def sanitize_folder_name(name):
    # Remove or replace characters that aren't allowed in folder names
    return re.sub(r'[\\/*?:"<>|\]\[]', "", name).strip()

def normalize_title(text : str) -> str:
    return text.lower().strip()

def download_image(url : str, folder : str, filename : str, retries:int=3, delay:int=5) -> None:
    attempt = 0
    while attempt < retries:
        try:
            r = requests.get(url, stream=True, timeout=10, verify=False)

            r.raise_for_status()
            os.makedirs(folder, exist_ok=True)
            with open(os.path.join(folder, filename), 'wb') as f:
                for chunk in r.iter_content(1024):
                    f.write(chunk)
            print(f"✅ Downloaded: {filename}")
            return
        except RequestException as e:
            attempt += 1
            print(f"⚠️ Attempt {attempt} failed: {e}")
            if attempt < retries:
                print(f"🔁 Retrying in {delay} seconds...")
                time.sleep(delay)
            else:
                print(f"❌ Failed to download {filename} after {retries} attempts.")

def get_manga_image_lmanga(df: pl.DataFrame):
    titles = df.select("English Title").to_series().to_list()
    japanese_titles = df.select("Japanese Title").to_series().to_list()
    
    struct = {
        'jap_title': pl.String,
        'eng_title': pl.String,
        'found': pl.Boolean,
        'score': pl.Int8,
    }

    entries = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=False, args=[f"--disable-extensions-except={ublock}", f"--load-extension={ublock}"])
        page = browser.new_page()

        for eng, jap in zip(titles, japanese_titles):
            for attempt_title in [jap]:
                if not attempt_title or attempt_title.strip() == "":
                    continue

                print(f"\n🔍 Searching: {attempt_title} with english title {eng}")
                page.goto("https://www.lmanga.com/")
                page.wait_for_load_state("networkidle")
                time.sleep(random.uniform(1.5, 3))

                search_box = page.locator('xpath=//*[@id="Nav"]/div/div/div[2]/form/div/input')
                search_box.fill(attempt_title)
                search_box.press("Enter")
                page.wait_for_timeout(3000)

                results = page.locator("div.top-15")
                result_count = results.count()

                best_match = None
                best_score = 0

                for i in range(result_count):
                    result = results.nth(i)
                    title_el = result.locator("b")

                    if not title_el.is_visible():
                        continue

                    result_tile : str = ""
                    try:
                        text_context = title_el.text_content()
                        if text_context:
                            result_title = text_context.strip()
                        else:
                            continue
                    except Exception as e:
                        print(f"⚠️ Skipping result {i}: {e}")
                        continue

                    score = fuzz.partial_ratio(normalize_title(attempt_title), normalize_title(result_title))
                    if score > best_score:
                        best_match = result
                        best_score = score

                    if score > 97:
                        break

                if best_match and best_score >= 100:
                    print(f"✅ Match found for '{attempt_title}' with score {best_score}")
                    
                    try:
                        text_context = best_match.locator("b").text_content()
                        if text_context:
                            result_title = text_context.strip()
                        else:
                            continue
                    except Exception as e:
                        print(f"⚠️ Error getting text_context for the best match : {e}")
                        continue

                    matched_title = text_context.strip()
                    folder_path = os.path.join("mangas", eng)
                    Path(folder_path).mkdir(parents=True, exist_ok=True)

                    manga_href = best_match.locator("a.SeriesName").first.get_attribute("href")
                    manga_url = f"https://www.lmanga.com{manga_href}"
                    page.goto(manga_url, wait_until="networkidle")
                    time.sleep(random.uniform(1.5, 3))

                    chapters = page.locator("a.ChapterLink")
                    if chapters.count() == 0:
                        print("❌ No chapters found.")
                        break

                    random_chapter = random.randint(0, chapters.count() - 1)
                    ch = chapters.nth(random_chapter)
                    ch_url = f"https://www.lmanga.com{ch.get_attribute('href')}"
                    print(f"📖 Opening random chapter: {ch_url}")
                    page.goto(ch_url, wait_until="networkidle")
                    time.sleep(random.uniform(1.5, 3))

                    imgs = page.locator("img.img-fluid")
                    img_count = imgs.count()
                    if img_count == 0:
                        print("❌ No images found in chapter.")
                        break

                    img_indices = random.sample(range(img_count), min(3, img_count))  # up to 3 random images

                    for idx in img_indices:
                        img = imgs.nth(idx)
                        img_url = img.get_attribute("data-original") or img.get_attribute("src")
                        if not img_url:
                            print(f"❌ No valid image URL at index {idx}.")
                            continue

                        filename = os.path.basename(img_url.split("?")[0])
                        download_image(img_url, folder_path, filename)
                    
                    entries.append(
                        {
                            'eng_title': eng,
                            'jap_title': jap,
                            'score': best_score,
                            'found': True
                        }
                    )

                    time.sleep(random.uniform(3, 5))

                    break
                else:
                    print(f"❌ No good match with '{attempt_title}':'{eng}'")
                    entries.append(
                        {
                            'eng_title': eng,
                            'jap_title': jap,
                            'score': best_score,
                            'found': False
                        }
                    )
        
        pl.DataFrame(entries).write_parquet("./results.parquet")
        pl.DataFrame(entries).write_csv("./results.csv")

        input("\nDone. Press Enter to close the browser...")
        browser.close()

def get_manga_image_klz9(df: pl.DataFrame):
    titles = df.select("English Title").to_series().to_list()
    japanese_titles = df.select("Japanese Title").to_series().to_list()
    
    struct = {
        'jap_title': pl.String,
        'eng_title': pl.String,
        'found': pl.Boolean,
        'score': pl.Int8,
    }

    entries = []
    
    with sync_playwright() as p:
        # browser = p.chromium.launch_persistent_context(
        #     user_data_dir=user_data_dir,
        #     args=[
        #         f"--disable-features=ExtensionDisableUnsupportedDeveloper",
        #         f"--disable-extensions-except={ublock}",
        #         f"--load-extension={ublock}",
        #     ],
        #     headless=False,
        # )
        browser = p.chromium.launch(headless=False, executable_path='/Applications/Brave Browser.app/Contents/MacOS/Brave Browser')
        page = browser.new_page()
        page.wait_for_timeout(1000) # wait until uBlock Origin is initialised

        # browser = p.firefox.launch_persistent_context(
        #     user_data_dir=user_data_dir,
        #     channel='firefox',
        #     args=[
        #         f"--disable-extensions-except={ublock}",
        #         f"--load-extension={ublock}",
        #     ],
        #     headless=False,
        # )

        for eng, jap in zip(titles, japanese_titles):
            for attempt_title in [jap]:
                if not attempt_title or attempt_title.strip() == "":
                    continue

                print(f"\n🔍 Searching: {attempt_title} with english title {eng}")
                page.goto("https://klz9.com/idx")
                page.wait_for_load_state("networkidle")
                time.sleep(random.uniform(1.5, 3))

                search_box = page.locator('xpath=//*[@id="search"]')
                search_box.fill(attempt_title)
                search_box.press("Enter")
                page.wait_for_timeout(3000)

                results = page.locator("div.bodythumb")
                result_count = results.count()

                best_match = None
                best_score = 0

                for i in range(result_count):
                    result = results.nth(i)
                    title_el = result.locator("b")

                    if not title_el.is_visible():
                        continue
                    
                    result_tile : str = ""
                    try:
                        text_context = title_el.text_content()
                        if text_context:
                            result_title = text_context.strip()
                        else:
                            continue
                    except Exception as e:
                        print(f"⚠️ Skipping result {i}: {e}")
                        continue

                    score = fuzz.partial_ratio(normalize_title(attempt_title), normalize_title(result_title))
                    if score > best_score:
                        best_match = result
                        best_score = score

                    if score > 97:
                        break

                if best_match and best_score >= 100:
                    print(f"✅ Match found for '{attempt_title}' with score {best_score}")

                    matched_title = best_match.locator("b").text_content().strip()
                    folder_path = os.path.join("mangas", eng)
                    Path(folder_path).mkdir(parents=True, exist_ok=True)

                    manga_href = best_match.locator("a.SeriesName").first.get_attribute("href")
                    manga_url = f"https://www.lmanga.com{manga_href}"
                    page.goto(manga_url, wait_until="networkidle")
                    time.sleep(random.uniform(1.5, 3))

                    chapters = page.locator("a.ChapterLink")
                    if chapters.count() == 0:
                        print("❌ No chapters found.")
                        break

                    random_chapter = random.randint(0, chapters.count() - 1)
                    ch = chapters.nth(random_chapter)
                    ch_url = f"https://www.lmanga.com{ch.get_attribute('href')}"
                    print(f"📖 Opening random chapter: {ch_url}")
                    page.goto(ch_url, wait_until="networkidle")
                    time.sleep(random.uniform(1.5, 3))

                    imgs = page.locator("img.img-fluid")
                    img_count = imgs.count()
                    if img_count == 0:
                        print("❌ No images found in chapter.")
                        break

                    img_indices = random.sample(range(img_count), min(3, img_count))  # up to 3 random images

                    for idx in img_indices:
                        img = imgs.nth(idx)
                        img_url = img.get_attribute("data-original") or img.get_attribute("src")
                        if not img_url:
                            print(f"❌ No valid image URL at index {idx}.")
                            continue

                        filename = os.path.basename(img_url.split("?")[0])
                        download_image(img_url, folder_path, filename)
                    
                    entries.append(
                        {
                            'eng_title': eng,
                            'jap_title': jap,
                            'score': best_score,
                            'found': True
                        }
                    )

                    time.sleep(random.uniform(3, 5))

                    break
                else:
                    print(f"❌ No good match with '{attempt_title}':'{eng}'")
                    entries.append(
                        {
                            'eng_title': eng,
                            'jap_title': jap,
                            'score': best_score,
                            'found': False
                        }
                    )
        
        pl.DataFrame(entries).write_parquet("./results.parquet")
        pl.DataFrame(entries).write_csv("./results.csv")

        input("\nDone. Press Enter to close the browser...")
        browser.close()

    return 0
def get_df_from_csv(csv_addr :str = "top_manga.csv") -> pl.DataFrame:
    try:
        df = pl.read_csv(csv_addr)
        return df
    except Exception as e:
        print("Error reading csv, verify address.")
        return pl.DataFrame()
    
def main():
    # create_initial_csv()
    # create_folders_from_csv()

    df : pl.DataFrame = get_df_from_csv("./top_manga.csv")
    if df.is_empty():
        print("No CSV data found. Exiting")
        exit(0)

    get_manga_image_klz9(df)

if __name__ == "__main__":
    main()
