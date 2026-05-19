using System.Net;
using System.Net.Sockets;
using System.Text;
using UnityEngine;
using System.IO;
using System.Collections.Generic;

public class Tim
{
    public List<float> t;
}

public class Server : MonoBehaviour
{
    UdpClient udpServer;
    IPEndPoint remoteEP, lewy, prawy; // Tu zapisze siê adres klienta, który "zapuka"

    public int song = 0;
    public ObstaclesGen leftGen, rightGen;
    public PlayerController leftPlayer, rightPlayer;
    public LivesManager livesLeft, livesRight;

    void Start()
    {
        udpServer = new UdpClient(7777); // Nas³uchujemy na porcie 7777
        remoteEP = new IPEndPoint(IPAddress.Any, 0);
        Debug.Log("Serwer wystartowa³...");
    }

    void StartGame()
    {
        List<float> times = JsonUtility.FromJson<Tim>(File.ReadAllText(Application.persistentDataPath + "/" + song + ".json")).t;
        List<float> temp = new List<float>(times);
        leftGen.StartGame(temp, song);
        temp = new List<float>(times);
        rightGen.StartGame(temp, song);
    }

    public void Lose(bool isLeft)
    {
        if (win != 0) return;
        Debug.Log(isLeft);
        if (isLeft) win = 2;
        else win = 1;
        //return who won to python
    }

    float win = 0;
    int ready = 0;
    void Update()
    {
        if (udpServer.Available > 0)
        {
            byte[] data = udpServer.Receive(ref remoteEP);
            string input = Encoding.ASCII.GetString(data);

            if (lewy == null)
            {
                lewy = remoteEP;
                Debug.Log("Player " + ready + " connected");
                ready++;
            }
            else if (prawy == null && remoteEP.ToString() != lewy.ToString())
            {
                Debug.Log(lewy.ToString() + " " + remoteEP.ToString());
                prawy = remoteEP;
                Debug.Log("Player " + ready + " connected");
                ready++;
            }

            if (ready < 2) return;
            else if(ready == 2)
            {
                StartGame();
                ready = 3;
                return;
            }
            //Debug.Log($"Odebrano input: {input} od {remoteEP}");
            if (input == "A" && remoteEP.ToString() == lewy.ToString() && win == 0)
                leftPlayer.Move(0);
            else if(input == "D" && remoteEP.ToString() == lewy.ToString() && win == 0)
                leftPlayer.Move(1);
            else if(input == "A" && remoteEP.ToString() == prawy.ToString() && win == 0)
                rightPlayer.Move(0);
            else if(input == "D" && remoteEP.ToString() == prawy.ToString() && win == 0)
                rightPlayer.Move(1);
            if(input != "N")
            Debug.Log(input);

            float hearts1 = livesLeft.lives, hearts2 = livesRight.lives, pos1 = leftPlayer.transform.position.x,
                pos2 = rightPlayer.transform.position.x;

            string response = $"{pos1}.{pos2}.{hearts1}.{hearts2}.{win}";
            byte[] responseData = Encoding.ASCII.GetBytes(response);
            udpServer.Send(responseData, responseData.Length, remoteEP);
        }
    }
}
//ODBIÓR: position1,position2,hearts1,hearts2,win(0->trwa, 1->leftWon, 2->rightWon)
//wysy³ka: A/N/D