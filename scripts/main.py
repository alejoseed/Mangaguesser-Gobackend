import csv
import requests
from lxml import html
import re
import os
from playwright.sync_api import sync_playwright, Playwright, Locator
import polars as pl
from rapidfuzz import fuzz
import random
import time
import urllib3
from dataclasses import dataclass
from sys import platform
import certifi
import ssl
import re

# const pathToExtension = require('path').join(__dirname, 'my-extension');

ublock = "/Users/alejoseed/Projects/Mangaguesser-Gobackend/scripts/uBlock0.chromium"
# ublock = "/Users/alejoseed/Projects/Mangaguesser-Gobackend/scripts/uBlock0_1.64.1b8.firefox.signed.xpi"

user_data_dir = "/tmp/test-user-data-dir"

http = urllib3.PoolManager(cert_reqs="CERT_REQUIRED", ca_certs=certifi.where())

from pathlib import Path
from requests.exceptions import RequestException

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


@dataclass
class BestMatchTitle:
    obj: Locator | None
    title: str
    score: int


if platform == "linux":
    browser_path = "/usr/bin/chromium"
elif platform == "darwin":
    browser_path = "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
elif platform == "win32":
    print("havent tested windows so no browser is available.")
    exit(1)


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
                    manga_page.encoding = "utf-8"
                    manga_tree = html.fromstring(manga_page.text)
                    jp_node = manga_tree.xpath(
                        '//div[@class="spaceit_pad"]/span[text()="Japanese:"]/following-sibling::text()'
                    )
                    if jp_node:
                        japanese_title = jp_node[0].strip()
                except Exception as e:
                    print(f"Failed to fetch Japanese title for {english_title}: {e}")

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


def append_to_csv(filename, data):
    file_exists = os.path.isfile(filename)
    file_empty = not file_exists or (file_exists and os.path.getsize(filename) == 0)

    with open(filename, "a", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)

        if file_empty:
            writer.writerow(["eng_title", "jap_title", "score"])

        writer.writerow([data["eng_title"], data["jap_title"], data["score"]])


def normalize_title(text: str) -> str:
    return text.lower().strip()


def download_image(
    url: str, folder: str, filename: str, retries: int = 3, delay: int = 5
) -> None:
    attempt = 0
    while attempt < retries:
        try:
            r = requests.get(url, stream=True, timeout=10, verify=False)

            r.raise_for_status()
            os.makedirs(folder, exist_ok=True)
            with open(os.path.join(folder, filename), "wb") as f:
                for chunk in r.iter_content(1024):
                    f.write(chunk)
            print(f"Downloaded: {filename}")
            return
        except RequestException as e:
            attempt += 1
            print(f"Attempt {attempt} failed: {e}")
            if attempt < retries:
                print(f"Retrying in {delay} seconds...")
                time.sleep(delay)
            else:
                print(f"Failed to download {filename} after {retries} attempts.")


def get_manga_href(page, target_title) -> tuple[str, int] | None:
    container = page.locator("#search-results")

    cards = container.locator("> article").all()

    best_match = {"href": "", "score": 0}

    for card in cards:
        link_container = card.locator(
            "div.text-lg.font-semibold.flex.items-center.gap-1"
        )
        anchor = link_container.locator("a.link-hover")

        if anchor.count() > 0:
            result_title = anchor.inner_text().strip()
            result_href = anchor.get_attribute("href")

            print(f"Title: {result_title}")
            print(f"Link: {result_href}")
            score = fuzz.token_sort_ratio(target_title.lower(), result_title.lower())

            if score > 97:
                return (str(result_href), int(score))

            if score > best_match["score"]:
                best_match = {"href": result_href, "score": score}

    if best_match["score"] > 70:
        return (best_match["href"], best_match["score"])
    else:
        return None


def get_manga_image_weebcentral(
    df: pl.DataFrame, found_mangas: list[dict[str, object]]
):
    titles: list[str] = df.select("search_title").to_series().to_list()
    japanese_titles: list[str] = df.select("Japanese Title").to_series().to_list()
    original_english_titles: list[str] = (
        df.select("English Title").to_series().to_list()
    )
    entries: list[dict[str, object]] = found_mangas

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=False, executable_path=browser_path)

        context = browser.new_context()

        page = context.new_page()
        base_url = "https://weebcentral.com/search?text="

        rest_of_url = "&sort=Best+Match&order=Descending&official=Any&anime=Any&adult=Any&display_mode=Full+Display"

        for search_title, original_title, jap in zip(
            titles, original_english_titles, japanese_titles
        ):
            time.sleep(random.uniform(0.5, 3))
            for attempt_title in [search_title]:
                if not attempt_title or attempt_title.strip() == "":
                    continue
                if attempt_title == "Nausica of the Valley of the Wind":
                    print("Attention here..")

                print(
                    f"\n🔍 Searching: {attempt_title} with english title {original_title}"
                )
                try:
                    _ = page.goto(f"{base_url}{search_title}{rest_of_url}")
                except Exception as e:
                    print("Could not get to page. exiting", e)
                    exit(0)

                page.wait_for_selector("#search-results", timeout=5000)

                manga_data = get_manga_href(page, search_title)
                if manga_data:
                    print(f"Match found for '{attempt_title}")
                    folder_path = os.path.join("mangas", original_title)
                    Path(folder_path).mkdir(parents=True, exist_ok=True)

                    _ = page.goto(manga_data[0])

                    chapter_list = page.locator("#chapter-list")
                    all_chapters_btn = page.locator(
                        'xpath=//*[@id="chapter-list"]/button'
                    )
                    if all_chapters_btn.count() > 0:
                        all_chapters_btn.click()

                    time.sleep(random.uniform(0.5, 1))

                    chapters = chapter_list.locator("div")
                    if chapters.count() == 0:
                        print("No chapters found.")
                        break

                    random_chapter = random.randint(0, chapters.count() - 1)
                    ch = chapters.nth(random_chapter)
                    print(f"Opening random chapter: {random_chapter}")
                    ch.click()
                    time.sleep(random.uniform(1.0, 2))

                    imgs = page.locator("xpath=/html/body/main/section[3]//img")
                    img_count = imgs.count()
                    if img_count == 0:
                        print("No images found in chapter.")
                        break

                    img_indices = random.sample(range(img_count), min(3, img_count))

                    for idx in img_indices:
                        img = imgs.nth(idx)
                        img_url = img.get_attribute(
                            "data-original"
                        ) or img.get_attribute("src")
                        if not img_url:
                            print(f"No valid image URL at index {idx}.")
                            continue

                        filename = os.path.basename(img_url.split("?")[0])
                        download_image(img_url, folder_path, filename)

                    entries.append(
                        {"eng_title": original_title, "jap_title": jap, "score": manga_data[1]}
                    )

                    csv_data = {
                        "eng_title": original_title,
                        "jap_title": jap,
                        "score": manga_data[1],
                    }
                    append_to_csv("./results.csv", csv_data)

                    time.sleep(random.uniform(3, 5))

                    break
                else:
                    print(f"No good match with '{attempt_title}':'{original_title}'")

        pl.DataFrame(entries).write_csv("./results.csv")

        input("\nDone. Press Enter to close the browser...")
        browser.close()

    return 0


def main():
    # create_initial_csv()
    # create_folders_from_csv()

    df: pl.DataFrame = pl.read_csv(
        "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/top_manga_gemini.csv"
    )
    filter: pl.DataFrame = pl.read_csv(
        "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/results.csv"
    )

    found: list[str] = filter.select("eng_title").to_series().to_list()
    found_mangas = filter.to_dicts()

    df = df.filter(~pl.col("English Title").is_in(found))

    if df.is_empty():
        print("No CSV data found. Exiting")
        exit(0)

    df = df.with_columns(
        pl.col("English Title")
        .str.replace_all("[^a-zA-Z0-9]", " ")
        .alias("search_title")
    ).with_columns(pl.col("search_title").str.normalize("NFKC"))

    # get_manga_image_klz9(df)
    get_manga_image_weebcentral(df, found_mangas)


if __name__ == "__main__":
    main()
