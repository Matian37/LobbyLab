using System.Net;
using System.Net.Sockets;
using System.Text;
using UnityEngine;

public class Client : MonoBehaviour
{
    UdpClient client;
    IPEndPoint serverEP;

    public Transform player1, player2;
    public GameObject[] lives1, lives2;
    public ObstaclesGen man;

    void Start()
    {
        client = new UdpClient();
        // Adres serwera (na razie localhost) i port
        serverEP = new IPEndPoint(IPAddress.Parse("127.0.0.1"), 7777);
        Time.timeScale = 0;
    }

    void Update()
    {
        // 1. Wysy³anie inputu (np. po klikniêciu spacji)
        byte[] data;
        if (Input.GetKeyDown(KeyCode.A))
            data = Encoding.ASCII.GetBytes("A");
        else if (Input.GetKeyDown(KeyCode.D))
            data = Encoding.ASCII.GetBytes("D");
        else
            data = Encoding.ASCII.GetBytes("N");
        client.Send(data, data.Length, serverEP);

        // 2. Odbieranie pozycji od serwera
        if (client.Available > 0)
        {
            IPEndPoint senderEP = new IPEndPoint(IPAddress.Any, 0);
            byte[] receivedData = client.Receive(ref senderEP);
            string message = Encoding.ASCII.GetString(receivedData);
            if (message == "0" || message == "" || message == null)
                return;
            if (Time.timeScale == 0) Time.timeScale = 1;
            string[] floats = message.Split('.');
            float pos1 = float.Parse(floats[0]);
            float pos2 = float.Parse(floats[1]);
            float hearts1 = float.Parse(floats[2]);
            float hearts2 = float.Parse(floats[3]);
            float win = float.Parse(floats[4]);
            Debug.Log(pos2 + " " + message);
            player1.position = new Vector3(pos1, player1.position.y, player1.position.z);
            player2.position = new Vector3(pos2, player2.position.y, player2.position.z);
            int before = 0;
            for (int i = 0; i < lives1.Length; i++)
                if (lives1[i].activeInHierarchy)
                    before++;
            for(int i = 0; i < lives1.Length; i++)
            {
                lives1[i].SetActive(false);
                lives2[i].SetActive(false);
            }
            for (int i = 0; i < hearts1; i++)
                lives1[i].SetActive(true);
            for (int i = 0; i < hearts2; i++)
                lives2[i].SetActive(true);
            if(hearts1 < before)
                SkyBoxManager.Instance.ChangeSky(SkyBoxManager.Instance.red, true, 4, true, false);
            if (win == 1)
                man.MultEndGame("Wygra³eœ!");
            else if (win == 2)
                man.MultEndGame("Przegra³eœ!");
        }
    }
}

//ODBIÓR: position1,position2,hearts1,hearts2,win(0->trwa, 1->youWin, 2->youLose)
//wysy³ka: A/N/D