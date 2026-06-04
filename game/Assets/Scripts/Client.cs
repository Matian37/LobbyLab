using System;
using System.Collections;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Text;
using TMPro;
using UnityEditor.PackageManager;
using UnityEngine;
using UnityEngine.Networking;

public class  WaitingUserData
{
    public string token;
    public WaitingUserData(string _token)
    {
        token = _token;
    }
}

public class SSE
{
    public string ip;
    public int port;
}

public class Client : MonoBehaviour
{
    UdpClient client;
    IPEndPoint serverEP;
    string url = "https://strona.pl/api/connections";

    public Transform player1, player2;
    public GameObject[] lives1, lives2;
    public GameManager man;
    public TextMeshProUGUI[] nicksText;
    public TextMeshProUGUI countdownText;
    public GameObject waitingArea;
    public Transform obstacleParent;
    public ObstaclesGen leftObstacleGen, rightObstacleGen;

    private WebClient klient;
    private StreamReader czytnik;
    bool gotServerSocket = false;

    void Start()
    {
        AddToWaitingList();

        //open sse source
        try
        {
            klient = new WebClient();
            // Otwieramy strumieñ GET pod wskazanym adresem
            Stream strumien = klient.OpenRead(url + $"?login={UserAccountData.Instance.login}");
            czytnik = new StreamReader(strumien);

            Debug.Log("Po³¹czono z SSE!");
        }
        catch (Exception e)
        {
            Debug.LogError("B³¹d po³¹czenia: " + e.Message);
        }

    }

    void OnDestroy()
    {
        // Sprz¹tamy po wy³¹czeniu gry
        if (czytnik != null) czytnik.Close();
        if (klient != null) klient.Dispose();
    }

    void AddToWaitingList()
    {
        string url = "http://localhost:5173/waiting";

        WaitingUserData dane = new WaitingUserData(UserAccountData.Instance.token);
        StartCoroutine(ApiSender.Instance.SendQuery(dane, url, (jsonWynik) =>
        {
            if (jsonWynik != null)
            {
                Debug.Log("jestes na waitliscie");
            }
        }));
    }

    void Update()
    {
        // Sprawdzamy w ka¿dej klatce, czy serwer coœ przys³a³
        if (czytnik != null && !czytnik.EndOfStream)
        {
            string linia = czytnik.ReadLine();

            if (!string.IsNullOrEmpty(linia))
            {
                string jsonData = linia.Substring(5).Trim();
                if (jsonData == "ping") return;
                SSE sseData = JsonUtility.FromJson<SSE>(jsonData);
                client = new UdpClient();
                serverEP = new IPEndPoint(IPAddress.Parse(sseData.ip), sseData.port);
                gotServerSocket = true;
            }
        }

        if (!gotServerSocket)
            return;
        byte[] data;
        if (Input.GetKeyDown(KeyCode.A))
            data = Encoding.ASCII.GetBytes("A");
        else if (Input.GetKeyDown(KeyCode.D))
            data = Encoding.ASCII.GetBytes("D");
        else
            data = Encoding.ASCII.GetBytes("N");
        client.Send(data, data.Length, serverEP); //wysylam input klienta (lewo, prawo lub nic)

        if (client.Available > 0)
        {
            IPEndPoint senderEP = new IPEndPoint(IPAddress.Any, 0);
            byte[] receivedData = client.Receive(ref senderEP);
            string message = Encoding.ASCII.GetString(receivedData);
            if (message == "0" || message == "" || message == null) //gdy nic nie odpowiada serwer to znaczy ze nie ma drugiego gracza
                return;
            waitingArea.SetActive(false);
            //odiberam wszystkie potrzebne dane
            string[] floats = message.Split('.');
            float pos1 = float.Parse(floats[0]);
            float pos2 = float.Parse(floats[1]);
            float hearts1 = float.Parse(floats[2]);
            float hearts2 = float.Parse(floats[3]);
            float win = float.Parse(floats[4]);
            string nick1 = floats[5];
            string nick2 = floats[6];
            float countdownTime = float.Parse(floats[7]);
            float obstaclePos = float.Parse(floats[8]);

            //ustawiam swoje obiekty na otrzymane
            player1.position = new Vector3(pos1, player1.position.y, player1.position.z);
            player2.position = new Vector3(pos2, player2.position.y, player2.position.z);
            int before = 0;
            for (int i = 0; i < lives1.Length; i++)
                if (lives1[i].activeInHierarchy)
                    before++;
            for (int i = 0; i < lives1.Length; i++)
            {
                lives1[i].SetActive(false);
                lives2[i].SetActive(false);
            }
            for (int i = 0; i < hearts1; i++)
                lives1[i].SetActive(true);
            for (int i = 0; i < hearts2; i++)
                lives2[i].SetActive(true);
            if (hearts1 < before)
                SkyBoxManager.Instance.ChangeSky(SkyBoxManager.Instance.red, true, 4, true, false);
            if (win == 1)
                man.MultEndGame($"Wygra³ {nick1}!");
            else if (win == 2)
                man.MultEndGame($"Wygra³ {nick2}!");
            nicksText[0].text = nick1;
            nicksText[1].text = nick2;
            obstacleParent.position = new Vector3(0, 0, obstaclePos);

            if (countdownTime == -1 && !man.didGameStart) //gdy skonczylo sie odliczanie to generuje wszystko lokalnie
            {
                countdownText.gameObject.SetActive(false);
                leftObstacleGen.Do();
                rightObstacleGen.Do();
                man.didGameStart = true;
            }
            else
                countdownText.text = Mathf.Round(countdownTime).ToString();
        }
    }
}

//ODBIÓR: position1.position2.hearts1.hearts2.win(0->trwa, 1->leftWon, 2->rightWon).nickLeft.nickRight.time(-1->gdy po countdown).obsPos
//wysy³ka: A/N/D