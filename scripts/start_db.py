import os
import polars as pl
import sqlite3


def get_names_from_folder():
    folder_path = "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/mangas"
    return [
        d
        for d in os.listdir(folder_path)
        if os.path.isdir(os.path.join(folder_path, d))
    ]


def assign_image_names() -> dict[str, list[str]]:
    folder_path = "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/mangas"
    manga_names = get_names_from_folder()
    folders: dict[str, str] = {}

    for manga in manga_names:
        folders[manga] = os.path.join(folder_path, manga)

    folder_and_images: dict[str, list[str]] = {}

    for name, folder in folders.items():
        for file in os.listdir(folder):
            if file.lower().endswith(".png"):
                folder_and_images.setdefault(name, []).append(file)

    return folder_and_images


def insert_mangas_db(manga_names: list[str]):
    insert_query = "insert or ignore into mangas (name) values (?)"
    try:
        with sqlite3.connect(
            "/home/alejoseed/Projects/Mangaguesser-Gobackend/manga_images.db"
        ) as conn:
            cur = conn.cursor()
            manga_tuples = [(name,) for name in manga_names]
            cur.executemany(insert_query, manga_tuples)
            conn.commit()

            print(f"Successfully inserted {len(manga_names)} manga names.")
    except Exception as e:
        print("Could not insert mangas.")
        print(e)


def insert_images_db(manga_dict: dict[str, list[str]]):
    select_manga_id = "select id from mangas where name = ?"
    insert_image = "insert or ignore into images (image_name, manga_id) values (?, ?)"

    try:
        with sqlite3.connect(
            "/home/alejoseed/Projects/Mangaguesser-Gobackend/manga_images.db"
        ) as conn:
            cur = conn.cursor()

            image_data = []
            for manga_name, image_names in manga_dict.items():
                cur.execute(select_manga_id, (manga_name,))
                result = cur.fetchone()

                if result:
                    manga_id = result[0]
                    for image_name in image_names:
                        image_data.append((image_name, manga_id))
                else:
                    print(
                        f"Warning: Manga '{manga_name}' not found in database, skipping images"
                    )

            if image_data:
                cur.executemany(insert_image, image_data)
                conn.commit()
                print(f"Successfully inserted {len(image_data)} image names.")
            else:
                print("No images to insert.")

    except Exception as e:
        print("Could not insert images.")
        print(e)


def find_missing_folders():
    manga_names_found = pl.read_csv(
        "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/results.csv"
    )

    manga_names_folders = get_names_from_folder()

    manga_list = [
        title
        for title in manga_names_found["eng_title"].to_list()
        if title and title.strip()
    ]
    missing_folders = [name for name in manga_list if name not in manga_names_folders]

    print(f"Total manga in dataframe: {len(manga_list)}")
    print(f"Total folders: {len(manga_names_folders)}")

    assert len(manga_list) == len(manga_names_folders), (
        f"Size mismatch: {len(manga_list)} manga vs {len(manga_names_folders)} folders"
    )

    print("Manga names in dataframe but no folder exists:")
    for name in missing_folders:
        print(f"- {name}")

    return missing_folders


def get_manga_data():
    manga_names_found = pl.read_csv(
        "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/results.csv"
    )

    manga_names_folders = get_names_from_folder()

    manga_list = [
        title
        for title in manga_names_found["eng_title"].to_list()
        if title and title.strip()
    ]

    assert len(manga_list) == len(manga_names_folders), (
        f"Size mismatch: {len(manga_list)} manga vs {len(manga_names_folders)} folders"
    )

    insert_mangas_db(manga_list)
    images_dict = assign_image_names()
    insert_images_db(images_dict)


if __name__ == "__main__":
    get_manga_data()
