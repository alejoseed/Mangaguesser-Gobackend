import requests
from lxml import html

def main():
    url = "https://myanimelist.net/topmanga.php?type=manga"
    headers = {
        "User-Agent": "Mozilla/5.0"
    }

    # Fetch the page
    response = requests.get(url, headers=headers)
    response.raise_for_status()

    # Parse the page
    tree = html.fromstring(response.content)

    # Extract using XPath
    xpath = '//*[@id="#area2"]'
    element = tree.xpath(xpath)

    # Print the result
    if element:
        print("Title:", element[0].text_content())
        print("Link:", element[0].get("href"))
    else:
        print("Element not found.")

if __name__ == '__main__':
    main()