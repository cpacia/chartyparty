document.addEventListener("DOMContentLoaded", () => {
    console.log("DOM fully loaded and parsed");

    const landingPage = document.getElementById('landing-page');
    const gamePage = document.getElementById('game-page');
    const newGameButton = document.getElementById('new-game');
    const joinGameButton = document.getElementById('join-game');
    const usernameInput = document.getElementById('username');
    const gameIdInput = document.getElementById('game-id');
    const gameIdDisplay = document.getElementById('game-id-display');
    const selectButton = document.getElementById('select');
    const nextButton = document.getElementById('next');
    const cards = document.querySelectorAll('.card');
    let selectedCard = null;
    let submittedCard = null;
    let gameId = null;
    let username = null;
    let userID = null;

    newGameButton.addEventListener('click', () => {
        if (usernameInput.value === "") {
            alert("You must enter a name");
            return
        }
        username = usernameInput.value;
        const player1NameElement = document.getElementById('player1-name');
        player1NameElement.textContent = username;
        fetch(`/newgame?name=${username}`, { method: 'POST' })
            .then(response => response.json())
            .then(data => {
                gameId = data.gameID;
                userID = data.userID;
                startGame();
            });
    });

    joinGameButton.addEventListener('click', () => {
        if (usernameInput.value === "") {
            alert("You must enter a name");
            return
        }
        if (gameIdInput.value === "") {
            alert("You must enter a game ID");
            return
        }
        username = usernameInput.value;
        gameId = gameIdInput.value;
        const player2NameElement = document.getElementById('player2-name');
        player2NameElement.textContent = username;
        fetch(`/joingame?gameID=${gameId}&name=${username}`, { method: 'POST' })
            .then(response => response.json())
            .then(data => {
                const player1NameElement = document.getElementById('player1-name');
                player1NameElement.textContent = data.opponent;
                userID = data.userID;
                startGame();
            });
    });

    function startGame() {
        gameIdDisplay.textContent = `Game ID: ${gameId}`;
        landingPage.classList.remove('active');
        gamePage.classList.add('active');
        loadChart();
        loadCards();

        // Websocket setup
        const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${wsProtocol}//${window.location.host}/ws`;
        const socket = new WebSocket(wsUrl);
        socket.onopen = () => {
            socket.send(JSON.stringify({ join: {gameID: gameId, userID: userID} }));
        };
        socket.onmessage = (event) => {
            const data = JSON.parse(event.data);
            if (data.connect) {
                const player2NameElement = document.getElementById('player2-name');
                player2NameElement.textContent = data.connect;
            }
            if (data.submit) {
                console.log(data.submit);
                var playerImage = null;
                if (document.getElementById('player1-name').textContent === username) {
                    playerImage = document.getElementById('player2-image');
                } else {
                    playerImage = document.getElementById('player1-image');
                }
                playerImage.src = "/cards/card_"+data.submit+".jpg";
                playerImage.alt = "Selected Card";
                playerImage.classList.remove('hidden');
                if (!document.getElementById('player1-image').classList.contains('hidden') && !document.getElementById('player2-image').classList.contains('hidden')){
                    nextButton.disabled = false;
                }
            }
        };

        cards.forEach(card => {
            card.addEventListener('click', () => {
                if (selectedCard) {
                    selectedCard.classList.remove('selected');
                }
                selectedCard = card;
                card.classList.add('selected');
                selectButton.disabled = false;
            });
        });

        selectButton.addEventListener('click', () => {
            if (selectedCard) {
                var playerImage = null;
                if (document.getElementById('player1-name').textContent === username) {
                    playerImage = document.getElementById('player1-image');
                } else {
                    playerImage = document.getElementById('player2-image');
                }
                playerImage.src = selectedCard.querySelector('img').src;
                playerImage.alt = "Selected Card";
                playerImage.classList.remove('hidden');
                selectButton.disabled = true;

                fetch(`/submitcard?gameID=${gameId}&userID=${userID}&card=${selectedCard.dataset.id}`, { method: 'POST' })
                    .then(response => response.json())
                    .then(() => {
                        submittedCard = selectedCard.id;
                        selectedCard.classList.remove('selected');
                        selectButton.disabled = true;
                        if (!document.getElementById('player1-image').classList.contains('hidden') && !document.getElementById('player2-image').classList.contains('hidden')){
                            nextButton.disabled = false;
                        }
                    });
            }
        });

        nextButton.addEventListener('click', () => {
            // Logic for next round
            loadChart();
            nextButton.disabled = true;
            selectButton.disabled = false;
            selectedCard = null;

            document.getElementById('player2-image').classList.add("hidden");
            document.getElementById('player1-image').classList.add("hidden");

            fetch(`/card?gameID=${gameId}`)
                .then(response => response.json())
                .then(data => {
                    if (data.id) {
                        const cardElement = document.getElementById(submittedCard);
                        const existingImage = cardElement.querySelector('img');
                        if (existingImage) {
                            cardElement.removeChild(existingImage);
                        }
                        const img = document.createElement('img');
                        img.src = `/cards/card_${data.id}.jpg`;
                        img.alt = "Card Image";
                        img.className = 'card-image';
                        document.getElementById(submittedCard).appendChild(img);
                        cardElement.dataset.id = data.id;
                    } else {
                        console.error("No image URL returned by API");
                    }
                })
                .catch(error => console.error('Error fetching card:', error));
        });
    }

    function loadChart() {
        fetch(`/chart?gameID=${gameId}`)
            .then(response => response.json())
            .then(data => {
                const chartImage = document.getElementById('chart-image');
                if (data.id) {
                    chartImage.src = "/charts/chart_"+data.id+".jpg";
                    chartImage.alt = "Chart Image";
                } else {
                    console.error("No image URL returned by API");
                }
            });
    }

    function loadCards() {
        const cards = document.querySelectorAll('.card');

        cards.forEach((card, index) => {
            const cardImage = card.querySelector('img');

            // Check if the card is currently unset
            if (!cardImage || cardImage.src === '') {
                fetch(`/card?gameID=${gameId}`)
                    .then(response => response.json())
                    .then(data => {
                        if (data.id) {
                            const img = document.createElement('img');
                            img.src = `/cards/card_${data.id}.jpg`;
                            img.alt = "Card Image";
                            img.className = 'card-image';
                            card.appendChild(img);
                            card.dataset.id = data.id;
                        } else {
                            console.error("No image URL returned by API");
                        }
                    })
                    .catch(error => console.error('Error fetching card:', error));
            }
        });
    }
});
