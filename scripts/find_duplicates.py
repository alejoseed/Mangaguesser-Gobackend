#!/usr/bin/env python3
import polars as pl


def find_duplicates():
    df = pl.read_csv("results.csv")
    eng_titles = df["eng_title"].to_list()

    # Count occurrences
    title_counts = {}
    for title in eng_titles:
        if title and title.strip():
            title_counts[title] = title_counts.get(title, 0) + 1

    # Find duplicates
    duplicates = {title: count for title, count in title_counts.items() if count > 1}

    print("Duplicate manga titles in CSV:")
    for title, count in sorted(duplicates.items()):
        print(f"  - {title}: {count} times")

    print(f"\nTotal duplicates: {len(duplicates)}")
    print(f"Extra duplicate entries: {sum(count - 1 for count in duplicates.values())}")


if __name__ == "__main__":
    find_duplicates()
