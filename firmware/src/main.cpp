#include <Arduino.h>
#include <SPI.h>
#include <MFRC522.h>

// Auf false setzen wenn der ESP produktiv läuft (nur TAG:-Zeilen, kein Rauschen)
#define DEBUG_MODE false

#define PIN_RST  D3
#define PIN_SS   D8
#define PIN_BUZZ D1

MFRC522 rfid(PIN_SS, PIN_RST);

static unsigned long letzterHeartbeat = 0;

void beep(uint16_t dauer_ms) {
    digitalWrite(PIN_BUZZ, HIGH);
    delay(dauer_ms);
    digitalWrite(PIN_BUZZ, LOW);
}

// Scan-Bestätigung: di-di-DAAAH
void beepScan() {
    beep(60);  delay(60);
    beep(60);  delay(60);
    beep(300);
}

String uidAlsString(MFRC522::Uid& uid) {
    String s;
    for (byte i = 0; i < uid.size; i++) {
        if (i > 0) s += ' ';
        if (uid.uidByte[i] < 0x10) s += '0';
        s += String(uid.uidByte[i], HEX);
    }
    s.toUpperCase();
    return s;
}

void handleCard() {
    String uid = uidAlsString(rfid.uid);

    // Maschinenlesbare Ausgabe für die Go-Software – immer ausgeben
    Serial.print("TAG:");
    Serial.println(uid);

#if DEBUG_MODE
    Serial.println("┌─────────────────────────────────┐");
    Serial.print  ("│ RFID-Tag gelesen                 │\n│ UID: ");
    Serial.println(uid);
    Serial.println("└─────────────────────────────────┘");
#endif

    beepScan();
}

void setup() {
    Serial.begin(115200);
    delay(200);

    pinMode(PIN_BUZZ, OUTPUT);
    digitalWrite(PIN_BUZZ, LOW);

#if DEBUG_MODE
    Serial.println("\n=====================================");
    Serial.println("       Dewey RFID-Reader v1.0        ");
    Serial.print  ("       Modus: DEBUG");
    Serial.println("\n=====================================");
#endif

    SPI.begin();
    rfid.PCD_Init();
    delay(50);

    byte version = rfid.PCD_ReadRegister(MFRC522::VersionReg);

#if DEBUG_MODE
    if (version == 0x00 || version == 0xFF) {
        Serial.println("[FEHLER] RC522 nicht erkannt – SPI-Verkabelung prüfen!");
    } else {
        Serial.print("[OK] RC522 erkannt (Firmware 0x");
        Serial.print(version, HEX);
        Serial.println(")");
    }
    Serial.println("Warte auf Tag  (. = 3s Heartbeat)");
    Serial.println("-------------------------------------");
#endif
}

void loop() {
#if DEBUG_MODE
    if (millis() - letzterHeartbeat >= 3000) {
        Serial.print('.');
        letzterHeartbeat = millis();
    }
#endif

    if (!rfid.PICC_IsNewCardPresent() || !rfid.PICC_ReadCardSerial()) {
        delay(50);
        return;
    }

    handleCard();

    rfid.PICC_HaltA();
    rfid.PCD_StopCrypto1();

    delay(2000);
    letzterHeartbeat = millis();
}
