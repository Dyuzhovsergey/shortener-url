package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
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

	resp, err := http.Post(baseURL, "text/plain", strings.NewReader(longURL))
	if err != nil {
		fmt.Println("Ошибка при запросе:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Статус: %s\nОтвет сервера: %s\n", resp.Status, string(body))
}

func handleGet(baseURL string, reader *bufio.Reader) {
	fmt.Print("Введите короткий ID: ")
	id, _ := reader.ReadString('\n')
	id = strings.TrimSpace(id)

	client := &http.Client{
		// Запрещаем следовать за редиректами
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(baseURL + id)
	if err != nil {
		fmt.Println("Ошибка при запросе:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTemporaryRedirect ||
		resp.StatusCode == http.StatusPermanentRedirect {
		location := resp.Header.Get("Location")
		fmt.Println("Оригинальный URL:", location)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Статус: %s\nОтвет: %s\n", resp.Status, string(body))
}
