import os
import polars as pl

def get_names_from_folder():
    folder_path = "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/mangas"
    return [
        d
        for d in os.listdir(folder_path)
        if os.path.isdir(os.path.join(folder_path, d))
    ]


def assign_image_names():
    folder_path = "/home/alejoseed/Projects/Mangaguesser-Gobackend/scripts/mangas"
    folders = [os.path.join(folder_path, name) for name in get_names_from_folder()]

    png_files = []
    for folder in folders:
        for file in os.listdir(folder):
            if file.lower().endswith(".png"):
                png_files.append(os.path.join(folder, file))

    return png_files



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
    png_files = assign_image_names()
    print(f"Total PNG files found: {len(png_files)}")
    for png_file in png_files:
        print(png_file)
    return png_files


if __name__ == "__main__":
    get_manga_data()
