using System.Net;
using System.Net.Sockets;
using System.Text;
using UnityEngine;
using System.IO;
using System.Collections.Generic;
using System;

public class Tim
{
    public List<float> t;
}

public class Server : MonoBehaviour
{
    UdpClient udpServer;
    IPEndPoint remoteEP, lewy, prawy;

    //things we need as input
    public int song = 0;
    public string[] nicks;

    public ObstaclesGen leftGen, rightGen;
    public PlayerController leftPlayer, rightPlayer;
    public LivesManager livesLeft, livesRight;
    public Transform obstacleParent;

    void Start()
    {
        udpServer = new UdpClient(7777); // Nas³uchujemy na porcie 7777
        remoteEP = new IPEndPoint(IPAddress.Any, 0);
        Debug.Log("Serwer wystartowa³");
    }

    void StartGame()
    {
        List<float> times = JsonUtility.FromJson<Tim>(File.ReadAllText(Application.persistentDataPath + "/" + song + ".json")).t;
        List<float> temp = new List<float>(times);
        leftGen.StartGame(temp, song);
        temp = new List<float>(times);
        rightGen.StartGame(temp, song);
    }

    void StartCountdown()
    {
        isCountdown = true;
        time = 3;
    }

    public void Lose(bool isLeft)
    {
        if (win != 0) return;
        Debug.Log(isLeft);
        if (isLeft) win = 2;
        else win = 1;
        //return who won to python
    }

    bool isCountdown = false;
    float time = 3;
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
                Debug.Log("Player " + (ready+1) + " connected");
                ready++;
            }
            else if (prawy == null && remoteEP.ToString() != lewy.ToString())
            {
                Debug.Log(lewy.ToString() + " " + remoteEP.ToString());
                prawy = remoteEP;
                Debug.Log("Player " + (ready+1) + " connected");
                ready++;
            }

            if (ready < 2) return;
            else if(ready == 2)
            {
                Debug.Log("both players in, game starts");
                StartCountdown();
                ready = 3;
                return;
            }
            if(isCountdown)
            {
                time -= Time.deltaTime;
                if (time <= 0)
                {
                    isCountdown = false;
                    time = -1;
                    StartGame();
                    Debug.Log("countdown failed");
                }
            }

            if (input == "A" && remoteEP.ToString() == lewy.ToString() && win == 0 && !isCountdown)
                leftPlayer.Move(0);
            else if(input == "D" && remoteEP.ToString() == lewy.ToString() && win == 0 && !isCountdown)
                leftPlayer.Move(1);
            else if(input == "A" && remoteEP.ToString() == prawy.ToString() && win == 0 && !isCountdown)
                rightPlayer.Move(0);
            else if(input == "D" && remoteEP.ToString() == prawy.ToString() && win == 0 && !isCountdown)
                rightPlayer.Move(1);

            float hearts1 = livesLeft.lives, hearts2 = livesRight.lives, pos1 = leftPlayer.transform.position.x,
                pos2 = rightPlayer.transform.position.x;
            if (win != 0) obstacleParent.GetComponent<ElementControl>().Stop();
            string response = $"{pos1}.{pos2}.{hearts1}.{hearts2}.{win}.{nicks[0]}.{nicks[1]}.{time}.{obstacleParent.position.z}";
            byte[] responseData = Encoding.ASCII.GetBytes(response);
            udpServer.Send(responseData, responseData.Length, remoteEP);
        }
    }
}
//ODBIÓR: position1.position2.hearts1.hearts2.win(0->trwa, 1->leftWon, 2->rightWon).nickLeft.nickRight.time(-1->gdy po countdown)
//wysy³ka: A/N/D