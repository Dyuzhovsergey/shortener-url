package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func main() {
	baseURL := "http://localhost:8080/"

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\nВыберите действие:")
		fmt.Println("1 — Отправить длинный URL (POST)")
		fmt.Println("2 — Получить оригинальный URL (GET)")
		fmt.Println("0 — Выход")
		fmt.Print("Ваш выбор: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			handlePost(baseURL, reader)
		case "2":
			handleGet(baseURL, reader)
		case "0":
			fmt.Println("Выход из программы.")
			return
		default:
			fmt.Println("Неизвестная команда.")
		}
	}
}

func handlePost(baseURL string, reader *bufio.Reader) {
	fmt.Print("Введите длинный URL: ")
	longURL, _ := reader.ReadString('\n')
	longURL = strings.TrimSpace(longURL)

	data := url.Values{}
	data.Set("url", longURL)

	resp, err := http.PostForm(baseURL, data)
	if err != nil {
		fmt.Println("Ошибка при запросе:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Ответ сервера:", string(body))
}

func handleGet(baseURL string, reader *bufio.Reader) {
	fmt.Print("Введите короткий ID: ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)

	resp, err := http.Get(baseURL + id)
	if err != nil {
		fmt.Println("Ошибка при запросе:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Сервер вернул:", resp.Status)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Оригинальный URL:", string(body))
}
