const messages = [
    '(0) nie istnieje sesja z danym tokenem',
    '(1) nie dodano uzytkownika do bazy oczekujacych, prawdopodobnie juz tam jest',
    '(2) nie usunieto uzytkownika z bazy oczekujacych, prawdopodobnie nie bylo go tam',
    '(3) klient przerwal polaczenie SSE',
    '(4) proba usuniecia nieistniejacej sesji'
];

export function handleError(errorCode, customMessage = ""){
    if(errorCode < 0) console.debug(customMessage);
    else console.debug(messages[errorCode]);
}