#!/usr/bin/env python3
import polars as pl
import os


def get_manga_csv_list():
    """Get list of manga names from CSV"""
    df = pl.read_csv("results.csv")
    # Get unique English titles from both columns (some rows have duplicates)
    eng_titles = df["eng_title"].to_list()
    # Remove any None/empty values
    manga_list = [title for title in eng_titles if title and title.strip()]
    return list(set(manga_list))  # Remove duplicates


def get_manga_folders():
    """Get list of manga folder names"""
    folder_path = "mangas"
    return [
        d
        for d in os.listdir(folder_path)
        if os.path.isdir(os.path.join(folder_path, d))
    ]


def main():
    manga_list = get_manga_csv_list()
    manga_folders = get_manga_folders()

    print(f"Total unique manga in CSV: {len(manga_list)}")
    print(f"Total folders: {len(manga_folders)}")
    print()

    # Find manga in CSV but not in folders
    missing_from_folders = [name for name in manga_list if name not in manga_folders]

    # Find folders that don't match any manga in CSV
    extra_folders = [name for name in manga_folders if name not in manga_list]

    print(f"Manga in CSV but NO folder exists ({len(missing_from_folders)}):")
    for name in sorted(missing_from_folders):
        print(f"  - {name}")

    print(f"\nFolders that DON'T match any CSV entry ({len(extra_folders)}):")
    for name in sorted(extra_folders):
        print(f"  - {name}")

    print(f"\nDifference: {len(missing_from_folders)} more CSV entries than folders")


if __name__ == "__main__":
    main()
